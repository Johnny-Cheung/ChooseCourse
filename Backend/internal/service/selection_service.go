package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"choose-course-backend/internal/cache"
	"choose-course-backend/internal/model"
	"choose-course-backend/internal/pkg/logger"
	"choose-course-backend/internal/pkg/requestid"
	"choose-course-backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// 这两个常量表示“抢课请求表”里的动作类型。
	// M5 先做同步版，但我们仍然把动作类型统一成常量，
	// 这样后面做 M7 异步版时可以继续复用。
	selectionActionGrab = "grab"
	selectionActionDrop = "drop"

	// M5 同步版里，请求一旦执行成功，就直接记为 success。
	// 后面的异步版才会真正出现 pending 状态。
	selectionStatusSuccess = "success"
)

var (
	// ErrCourseClosed 表示课程存在，但当前不是开课状态，不能选。
	ErrCourseClosed = errors.New("course closed")
	// ErrCourseFull 表示课程人数已经满了。
	ErrCourseFull = errors.New("course full")
	// ErrCourseAlreadySelected 表示学生已经选过这门课。
	ErrCourseAlreadySelected = errors.New("course already selected")
	// ErrSelectionTimeConflict 表示待选课程与学生已选课程时间冲突。
	ErrSelectionTimeConflict = errors.New("selection time conflict")
	// ErrInsufficientCredits 表示学生剩余学分不足。
	ErrInsufficientCredits = errors.New("insufficient credits")
	// ErrCourseNotSelected 表示学生还没有选这门课，因此不能退课。
	ErrCourseNotSelected = errors.New("course not selected")
	// ErrSelectionRequestNotFound 表示按 request_no 没有查到请求记录。
	ErrSelectionRequestNotFound = errors.New("selection request not found")
)

// SelectionActionResult 是“抢课成功”或“退课成功”后返回给前端的数据。
//
// 这里把 request_no 一起返回，是为了让前端后续可以去查询该请求记录。
// 对 M5 来说它主要会是 success；到 M7 时它会更重要，因为那时会有 pending/failed。
type SelectionActionResult struct {
	RequestNo string `json:"request_no"` // 本次抢课/退课请求的唯一编号，前端后续可拿它去查请求记录
	Action    string `json:"action"`     // 本次动作类型：grab 表示抢课，drop 表示退课
	Status    string `json:"status"`     // 本次请求的处理结果状态；M5 同步版里通常是 success

	CourseID uint64 `json:"course_id"` // 本次操作对应的课程 ID

	SelectedCount int `json:"selected_count"` // 操作完成后，这门课当前的已选人数
	CreditUsed    int `json:"credit_used"`    // 操作完成后，这个学生当前已经使用掉的总学分
}

// SelectionRequestResult 是查询抢课请求状态接口返回的数据。
type SelectionRequestResult struct {
	RequestNo string `json:"request_no"` // 抢课请求编号，对应 selection_requests 表里的 request_no

	StudentID uint64 `json:"student_id"` // 发起这次请求的学生 ID
	CourseID  uint64 `json:"course_id"`  // 这次请求操作的目标课程 ID

	Action string `json:"action"` // 请求动作：grab 表示抢课，drop 表示退课
	Status string `json:"status"` // 请求当前状态：M5 里通常是 success；到 M7 会出现 pending/failed

	FailReason string `json:"fail_reason"` // 如果请求失败，这里记录失败原因；成功时通常为空字符串

	CreatedAt time.Time `json:"created_at"` // 这条请求记录最初创建的时间
	UpdatedAt time.Time `json:"updated_at"` // 这条请求记录最后一次更新的时间
}

// SelectionService 负责 M5 阶段与抢课、退课、请求状态有关的核心业务。
type SelectionService struct{}

// NewSelectionService 创建一个抢课服务实例。
func NewSelectionService() *SelectionService {
	return &SelectionService{}
}

// SelectCourse 执行同步事务版抢课。
//
// 这是 M5/M6 阶段最核心的函数之一。
//
// 在 M5 里，它只做 MySQL 事务版抢课。
// 到 M6 之后，它的执行顺序变成：
// 1. 先用 Redis 做高频预校验和预扣减
// 2. Redis 通过后，再进入 MySQL 事务做最终落库
// 3. 如果 MySQL 事务失败，就把 Redis 刚才的预扣减补偿回去
// 4. 如果 MySQL 成功，就直接返回成功结果
//
// 所以你可以把它理解成：
// - Redis 负责“快”
// - MySQL 负责“准”
func (s *SelectionService) SelectCourse(studentID, courseID uint64) (*SelectionActionResult, error) {
	requestNo := generateSelectionRequestNo()

	// 这里先准备一个 context，后面 Redis 缓存操作都会用到它。
	// 现在先用最简单的 Background；后面如果你想做得更细，可以再改成把请求上下文往 service 里传。
	ctx := context.Background()

	// M6 新增：先走 Redis 预校验。
	// 这一层会把“课程是否可选、是否满员、是否重复选、是否时间冲突、学分是否够”
	// 尽量放到 Redis 一次性完成。
	precheckCode, err := cache.PrecheckAndReserveSelection(ctx, studentID, courseID)
	if err != nil {
		return nil, err
	}

	// 如果 Lua 返回的不是 OK，而是某种业务失败码，
	// 这里把它统一转换成 service 层能理解的业务错误。
	if err := mapSelectPrecheckCode(precheckCode); err != nil {
		return nil, err
	}

	// result 先在事务外声明，等事务成功后再统一返回。
	var result *SelectionActionResult
	var notificationSpec *StudentNotificationSpec

	err = repository.DB().Transaction(func(tx *gorm.DB) error {
		// 先给学生行加锁。
		// 这样同一个学生同时发起两次抢课/退课时，事务会串行执行，避免学分和选课状态互相打架。
		var student model.Student
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&student, studentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStudentNotFound
			}

			return err
		}
		if student.Status != 1 {
			return ErrUserDisabled
		}

		// 再给课程行加锁。
		// 这样多人抢同一门课时，课程容量 selected_count 的更新会更安全。
		var course model.Course
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&course, courseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCourseNotFound
			}

			return err
		}

		// 课程存在，但如果当前不是开课状态，就不允许选。
		if course.Status != 1 {
			return ErrCourseClosed
		}

		// 人数已满就不能再选。
		if course.SelectedCount >= course.Capacity {
			return ErrCourseFull
		}

		// 查学生和这门课之间是否已经有选课记录。
		// 注意 enrollments 有联合唯一索引，所以一个学生和一门课只会对应一条记录。
		// 如果以前退过课，这里会查到 status=0 的旧记录，后面要做“恢复”而不是“重新插入”。
		var enrollment model.Enrollment
		enrollmentErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("student_id = ? AND course_id = ?", studentID, courseID).
			First(&enrollment).Error
		if enrollmentErr == nil && enrollment.Status == 1 {
			return ErrCourseAlreadySelected
		}
		if enrollmentErr != nil && !errors.Is(enrollmentErr, gorm.ErrRecordNotFound) {
			return enrollmentErr
		}

		// 时间冲突判断：
		// 找出这个学生所有“仍然有效(status=1)”的已选课程，
		// 看看它们的 time_slot 里是否已经占用了当前课程的时间片。
		var conflictCount int64
		if err := tx.Model(&model.Enrollment{}).
			Joins("JOIN courses ON courses.id = enrollments.course_id").
			Where("enrollments.student_id = ? AND enrollments.status = 1 AND courses.time_slot = ?", studentID, course.TimeSlot).
			Count(&conflictCount).Error; err != nil {
			return err
		}
		if conflictCount > 0 {
			return ErrSelectionTimeConflict
		}

		// 学分校验：
		// 选上这门课之后，学生已用学分不能超过总学分上限。
		newCreditUsed := student.CreditUsed + int(course.Credit)
		if newCreditUsed > student.CreditLimit {
			return ErrInsufficientCredits
		}

		// 先更新课程已选人数。
		newSelectedCount := course.SelectedCount + 1
		if err := tx.Model(&course).Update("selected_count", newSelectedCount).Error; err != nil {
			return err
		}

		// 再更新学生已用学分。
		if err := tx.Model(&student).Update("credit_used", newCreditUsed).Error; err != nil {
			return err
		}

		// 处理 enrollments 记录有两种情况：
		// 1. 以前从没选过：直接新建一条
		// 2. 以前选过又退了：把旧记录恢复成 status=1
		now := time.Now()
		if errors.Is(enrollmentErr, gorm.ErrRecordNotFound) {
			enrollment = model.Enrollment{
				StudentID:  studentID,
				CourseID:   courseID,
				Status:     1,
				SelectedAt: now,
				DroppedAt:  nil,
			}
			if err := tx.Create(&enrollment).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&enrollment).Updates(map[string]any{
				"status":      1,
				"selected_at": now,
				"dropped_at":  nil,
			}).Error; err != nil {
				return err
			}
		}

		// M5 同步版里也把请求记录写进 selection_requests，
		// 这样后面查询 request_no 时能拿到一条明确的 success 记录。
		if err := createSelectionRequest(tx, requestNo, studentID, courseID, selectionActionGrab, selectionStatusSuccess, ""); err != nil {
			return err
		}

		relatedCourseID := course.ID
		notificationSpec = newStudentNotificationSpec(
			newNotificationBizKey("course_select_success", requestNo),
			studentID,
			"course_select_success",
			"选课成功",
			fmt.Sprintf("你已成功选上课程《%s》。", course.CourseName),
			&relatedCourseID,
			nil,
		)

		// 组装最终返回结果。
		result = &SelectionActionResult{
			RequestNo:     requestNo,
			Action:        selectionActionGrab,
			Status:        selectionStatusSuccess,
			CourseID:      courseID,
			SelectedCount: newSelectedCount,
			CreditUsed:    newCreditUsed,
		}

		return nil
	})
	if err != nil {
		// 只要 MySQL 事务失败，就说明 Redis 里刚才那次“预扣减”不能保留。
		// 所以这里要把 Redis 状态补偿回去。
		if rollbackErr := cache.CompensateReservedSelection(ctx, studentID, courseID); rollbackErr != nil {
			// 如果补偿也失败，最安全的做法是把相关缓存删掉。
			// 这样下一次请求进来时，会自动回源 MySQL 重建缓存，避免脏缓存长期存在。
			if recoverErr := resetSelectionCaches(ctx, studentID, courseID); recoverErr != nil {
				return nil, fmt.Errorf("mysql transaction failed: %w; redis compensation failed: %v; cache reset failed: %v", err, rollbackErr, recoverErr)
			}
		}

		return nil, err
	}

	publishStudentNotificationBestEffort(ctx, notificationSpec)

	return result, nil
}

// DropCourse 执行同步事务版退课。
//
// 它和抢课的思路一样，也是一个事务包住整套流程：
// 1. 校验这门课确实是当前学生已选课程
// 2. 把 enrollment 改成已退
// 3. 回退课程人数和学生已用学分
// 4. 写请求记录和通知
func (s *SelectionService) DropCourse(studentID, courseID uint64) (*SelectionActionResult, error) {
	requestNo := generateSelectionRequestNo()
	ctx := context.Background()
	var result *SelectionActionResult
	var notificationSpec *StudentNotificationSpec

	err := repository.DB().Transaction(func(tx *gorm.DB) error {
		// 给学生行加锁，避免同一个学生并发执行多次退课/抢课导致学分异常。
		var student model.Student
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&student, studentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStudentNotFound
			}

			return err
		}
		if student.Status != 1 {
			return ErrUserDisabled
		}

		// 给课程行加锁，避免 selected_count 在并发退课时出现错乱。
		var course model.Course
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&course, courseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCourseNotFound
			}

			return err
		}

		// 查有效的选课记录。
		// 只有 status=1 的记录才说明“当前真的选着这门课”。
		var enrollment model.Enrollment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("student_id = ? AND course_id = ? AND status = 1", studentID, courseID).
			First(&enrollment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCourseNotSelected
			}

			return err
		}

		now := time.Now()
		if err := tx.Model(&enrollment).Updates(map[string]any{
			"status":     0,
			"dropped_at": now,
		}).Error; err != nil {
			return err
		}

		// 退课后课程人数减 1，但为了稳妥，这里不让它减成负数。
		newSelectedCount := course.SelectedCount
		if newSelectedCount > 0 {
			newSelectedCount--
		}
		if err := tx.Model(&course).Update("selected_count", newSelectedCount).Error; err != nil {
			return err
		}

		// 学生已用学分也要同步减掉这门课的学分。
		newCreditUsed := student.CreditUsed - int(course.Credit)
		if newCreditUsed < 0 {
			newCreditUsed = 0
		}
		if err := tx.Model(&student).Update("credit_used", newCreditUsed).Error; err != nil {
			return err
		}

		// 记录本次退课请求。
		if err := createSelectionRequest(tx, requestNo, studentID, courseID, selectionActionDrop, selectionStatusSuccess, ""); err != nil {
			return err
		}

		relatedCourseID := course.ID
		notificationSpec = newStudentNotificationSpec(
			newNotificationBizKey("course_drop_success", requestNo),
			studentID,
			"course_drop_success",
			"退课成功",
			fmt.Sprintf("你已成功退掉课程《%s》。", course.CourseName),
			&relatedCourseID,
			nil,
		)

		result = &SelectionActionResult{
			RequestNo:     requestNo,
			Action:        selectionActionDrop,
			Status:        selectionStatusSuccess,
			CourseID:      courseID,
			SelectedCount: newSelectedCount,
			CreditUsed:    newCreditUsed,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	publishStudentNotificationBestEffort(ctx, notificationSpec)

	// M6 新增：退课成功后，把 Redis 缓存刷新成最新数据库状态。
	//
	// 这里没有复用“Redis 预扣减回退脚本”的原因是：
	// 退课时 MySQL 已经提交成功，这时候最稳妥的做法是直接以 MySQL 为准，重建课程和学生缓存。
	refreshSelectionCachesAfterDrop(ctx, studentID, courseID)

	return result, nil
}

// GetSelectionRequest 查询当前学生自己的抢课请求记录。
//
// M5 同步版里，这个接口的主要作用是：
// 前端拿到 request_no 之后，可以再反查一次数据库里的记录。
func (s *SelectionService) GetSelectionRequest(studentID uint64, requestNo string) (*SelectionRequestResult, error) {
	trimmedRequestNo := strings.TrimSpace(requestNo)
	if trimmedRequestNo == "" {
		return nil, ErrSelectionRequestNotFound
	}

	var request model.SelectionRequest
	if err := repository.DB().
		Where("request_no = ? AND student_id = ?", trimmedRequestNo, studentID).
		First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSelectionRequestNotFound
		}

		return nil, err
	}

	return &SelectionRequestResult{
		RequestNo:  request.RequestNo,
		StudentID:  request.StudentID,
		CourseID:   request.CourseID,
		Action:     request.Action,
		Status:     request.Status,
		FailReason: request.FailReason,
		CreatedAt:  request.CreatedAt,
		UpdatedAt:  request.UpdatedAt,
	}, nil
}

// createSelectionRequest 是一个内部辅助函数，
// 用来往 selection_requests 表写一条请求记录。
func createSelectionRequest(
	tx *gorm.DB,
	requestNo string,
	studentID, courseID uint64,
	action, status, failReason string,
) error {
	record := model.SelectionRequest{
		RequestNo:  requestNo,
		StudentID:  studentID,
		CourseID:   courseID,
		Action:     action,
		Status:     status,
		FailReason: failReason,
	}

	return tx.Create(&record).Error
}

// generateSelectionRequestNo 生成本次抢课/退课请求的 request_no。
//
// 这里直接复用项目里现成的 requestid.Generate，
// 因为它已经具备“足够唯一”的能力，对这个个人项目完全够用。
func generateSelectionRequestNo() string {
	return requestid.Generate()
}

// mapSelectPrecheckCode 把 Redis Lua 返回的结果码转换成 service 层统一使用的业务错误。
func mapSelectPrecheckCode(code cache.SelectPrecheckCode) error {
	// 这个函数的作用很单纯：
	// 把 Redis 那边的“数字结果码”，翻译成 Go 代码里统一使用的业务错误。
	// 这样 handler 层就不需要知道 Redis 结果码的细节了。
	switch code {
	case cache.SelectPrecheckOK:
		return nil
	case cache.SelectPrecheckCourseClosed:
		return ErrCourseClosed
	case cache.SelectPrecheckCourseFull:
		return ErrCourseFull
	case cache.SelectPrecheckAlreadySelected:
		return ErrCourseAlreadySelected
	case cache.SelectPrecheckTimeConflict:
		return ErrSelectionTimeConflict
	case cache.SelectPrecheckInsufficientCredit:
		return ErrInsufficientCredits
	default:
		return fmt.Errorf("unexpected redis precheck code: %d", code)
	}
}

// resetSelectionCaches 删除课程和学生的相关缓存。
//
// 这是“最后的保底手段”：
// 当补偿失败，或者缓存状态已经不可信时，直接删掉缓存，
// 让下一次请求重新从 MySQL 回源加载。
func resetSelectionCaches(ctx context.Context, studentID, courseID uint64) error {
	// 先删课程缓存。
	if err := cache.InvalidateCourseSelectionCache(ctx, courseID); err != nil {
		return err
	}
	// 再删学生缓存。
	if err := cache.InvalidateStudentSelectionCache(ctx, studentID); err != nil {
		return err
	}

	return nil
}

// refreshSelectionCachesAfterDrop 在退课成功后，用 MySQL 最新状态刷新 Redis 缓存。
//
// 这里采用 best-effort 策略：
// - MySQL 已提交成功后，接口返回结果应以数据库为准
// - 缓存刷新失败只记录日志，不再向上返回错误
func refreshSelectionCachesAfterDrop(ctx context.Context, studentID, courseID uint64) {
	// 先按 MySQL 最新状态刷新课程缓存。
	if err := cache.RefreshCourseSelectionCache(ctx, courseID); err != nil {
		// 如果刷新失败，就别再相信当前缓存，直接删掉。
		if recoverErr := resetSelectionCaches(ctx, studentID, courseID); recoverErr != nil {
			logPostCommitCacheSyncFailure(
				"post-commit course cache refresh failed after drop",
				fmt.Errorf("%w; cache reset failed: %v", err, recoverErr),
				logger.Any("student_id", studentID),
				logger.Any("course_id", courseID),
			)
			return
		}

		logPostCommitCacheSyncFailure(
			"post-commit course cache refresh failed after drop; fallback invalidation succeeded",
			err,
			logger.Any("student_id", studentID),
			logger.Any("course_id", courseID),
		)
		return
	}

	// 再按 MySQL 最新状态刷新学生缓存。
	if err := cache.RefreshStudentSelectionCache(ctx, studentID); err != nil {
		// 同理，学生缓存刷新失败时也直接删掉，等待下次回源。
		if recoverErr := resetSelectionCaches(ctx, studentID, courseID); recoverErr != nil {
			logPostCommitCacheSyncFailure(
				"post-commit student cache refresh failed after drop",
				fmt.Errorf("%w; cache reset failed: %v", err, recoverErr),
				logger.Any("student_id", studentID),
				logger.Any("course_id", courseID),
			)
			return
		}

		logPostCommitCacheSyncFailure(
			"post-commit student cache refresh failed after drop; fallback invalidation succeeded",
			err,
			logger.Any("student_id", studentID),
			logger.Any("course_id", courseID),
		)
		return
	}
}
