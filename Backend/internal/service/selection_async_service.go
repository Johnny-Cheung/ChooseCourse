package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"choose-course-backend/internal/cache"
	"choose-course-backend/internal/model"
	"choose-course-backend/internal/mq"
	"choose-course-backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	selectionStatusPending = "pending"
	selectionStatusFailed  = "failed"

	// selectionPendingTimeout 表示一条异步抢课请求最多允许 pending 多久。
	//
	// 如果超过这个时间还没有被消费者处理完，
	// 我们就认为这条请求已经“异常卡住”，需要主动收口成 failed。
	selectionPendingTimeout = 2 * time.Minute

	// selectionPendingSweepInterval 表示后台扫描 pending 超时请求的频率。
	selectionPendingSweepInterval = 30 * time.Second

	selectionFailReasonTimeout = "selection request processing timeout"
)

// SelectionSubmitResult 是 M7 异步抢课接口“快速受理”后返回给前端的数据。
//
// 它和 M5/M6 的 SelectionActionResult 不同：
// - M5/M6 返回的是最终成功结果
// - M7 这里返回的是“请求已受理，正在处理中”
type SelectionSubmitResult struct {
	RequestNo string `json:"request_no"` // 本次抢课请求编号，前端后续要拿它查询最终状态
	Action    string `json:"action"`     // 当前动作类型，M7 第一版固定为 grab
	Status    string `json:"status"`     // 当前状态，提交成功后先返回 pending
	CourseID  uint64 `json:"course_id"`  // 目标课程 ID
}

// SelectionAsyncService 负责 M7 阶段的异步抢课流程：
// 1. 接口快速受理
// 2. 投递 MQ
// 3. 消费者异步落库
type SelectionAsyncService struct{}

// NewSelectionAsyncService 创建异步抢课服务实例。
func NewSelectionAsyncService() *SelectionAsyncService {
	return &SelectionAsyncService{}
}

// PendingTimeout 返回异步请求允许保持 pending 的最长时间。
//
// 单独做成方法，是为了让别的地方（例如 main.go 或测试代码）
// 不需要直接依赖这个文件里的常量名。
func (s *SelectionAsyncService) PendingTimeout() time.Duration {
	return selectionPendingTimeout
}

// PendingSweepInterval 返回后台扫描超时 pending 请求的时间间隔。
func (s *SelectionAsyncService) PendingSweepInterval() time.Duration {
	return selectionPendingSweepInterval
}

// SubmitSelectCourse 受理一次异步抢课请求。
//
// 这个函数运行在 HTTP 接口里，所以它的目标不是“马上把课抢完”，
// 而是：
// 1. 先用 Redis 做资格预校验
// 2. 生成 request_no
// 3. 写一条 pending 请求记录
// 4. 发布 MQ 消息
// 5. 快速返回 pending
func (s *SelectionAsyncService) SubmitSelectCourse(studentID, courseID uint64) (*SelectionSubmitResult, error) {
	requestNo := generateSelectionRequestNo()
	ctx := context.Background()

	precheckCode, err := cache.PrecheckAndReserveSelection(ctx, studentID, courseID)
	if err != nil {
		return nil, mapSelectionCacheError(err)
	}
	if err := mapSelectPrecheckCode(precheckCode); err != nil {
		return nil, err
	}

	// 先把请求记录写成 pending。
	// 这样即使后面接口已经返回，前端也能立刻根据 request_no 查询到一条存在的请求记录。
	request := model.SelectionRequest{
		RequestNo:  requestNo,
		StudentID:  studentID,
		CourseID:   courseID,
		Action:     selectionActionGrab,
		Status:     selectionStatusPending,
		FailReason: "",
	}
	if err := repository.DB().Create(&request).Error; err != nil {
		// pending 请求记录写失败时，说明这次受理根本没成功。
		// 此时 Redis 里的预扣减也不能保留。
		if rollbackErr := cache.CompensateReservedSelection(ctx, studentID, courseID); rollbackErr != nil {
			if recoverErr := resetSelectionCaches(ctx, studentID, courseID); recoverErr != nil {
				return nil, fmt.Errorf("create pending request failed: %w; redis compensation failed: %v; cache reset failed: %v", err, rollbackErr, recoverErr)
			}
		}
		return nil, err
	}

	message := mq.NewSelectionGrabMessage(requestNo, studentID, courseID)
	if err := mq.PublishSelectionGrab(ctx, message); err != nil {
		// 发布失败时，这次异步请求不算真正受理成功。
		// 所以要把请求记录标成 failed，并回退 Redis 预扣减。
		_ = failSelectionRequest(requestNo, fmt.Sprintf("publish selection message failed: %v", err))
		if rollbackErr := cache.CompensateReservedSelection(ctx, studentID, courseID); rollbackErr != nil {
			if recoverErr := resetSelectionCaches(ctx, studentID, courseID); recoverErr != nil {
				return nil, fmt.Errorf("publish selection message failed: %w; redis compensation failed: %v; cache reset failed: %v", err, rollbackErr, recoverErr)
			}
		}
		return nil, err
	}

	return &SelectionSubmitResult{
		RequestNo: requestNo,
		Action:    selectionActionGrab,
		Status:    selectionStatusPending,
		CourseID:  courseID,
	}, nil
}

// GetSelectionRequest 查询当前学生自己的异步抢课请求状态。
//
// 这里和原来直接“查表返回”相比，多做了一步：
// 如果发现这条请求 still pending 且已经超时，就先主动把它收口成 failed，
// 避免学生一直查到 pending。
func (s *SelectionAsyncService) GetSelectionRequest(studentID uint64, requestNo string) (*SelectionRequestResult, error) {
	trimmedRequestNo := strings.TrimSpace(requestNo)
	if trimmedRequestNo == "" {
		return nil, ErrSelectionRequestNotFound
	}

	// 先尝试对单条请求做一次“超时收口”。
	// 这一步是幂等的：如果它没超时、已经成功、已经失败，都会安全跳过。
	if _, err := s.tryFailTimedOutRequest(context.Background(), studentID, trimmedRequestNo); err != nil {
		return nil, err
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

// SweepTimedOutPendingRequests 扫描并收口“卡住太久的 pending 请求”。
//
// 它主要给后台定时任务使用。
// 返回值 count 表示本轮成功收口成 failed 的请求数量。
func (s *SelectionAsyncService) SweepTimedOutPendingRequests(ctx context.Context) (int, error) {
	deadline := time.Now().Add(-selectionPendingTimeout)

	// 先把“可能超时”的 request_no 列出来。
	// 后面会逐条带锁处理，避免和消费者并发更新同一条请求时互相打架。
	var candidates []struct {
		RequestNo string `gorm:"column:request_no"`
		StudentID uint64 `gorm:"column:student_id"`
	}
	if err := repository.DB().
		Model(&model.SelectionRequest{}).
		Select("request_no, student_id").
		Where("status = ? AND updated_at < ?", selectionStatusPending, deadline).
		Find(&candidates).Error; err != nil {
		return 0, err
	}

	count := 0
	for _, candidate := range candidates {
		changed, err := s.tryFailTimedOutRequest(ctx, candidate.StudentID, candidate.RequestNo)
		if err != nil {
			return count, err
		}
		if changed {
			count++
		}
	}

	return count, nil
}

// HandleGrabMessage 是 RabbitMQ 消费者真正调用的业务入口。
//
// 它的职责是：
// 1. 找到对应的 pending 请求记录
// 2. 做幂等检查
// 3. 执行最终 MySQL 落库
// 4. 成功则把请求改成 success
// 5. 失败则改成 failed，并做 Redis 补偿
func (s *SelectionAsyncService) HandleGrabMessage(ctx context.Context, message mq.SelectionMessage) error {
	// 先做最基础的消息内容校验。
	// 这里如果连 request_no / action 都不对，就说明这不是一条值得继续重试的正常消息，
	// 直接打成死信更合适。
	if message.RequestNo == "" {
		return mq.DeadLetter(fmt.Errorf("selection message missing request_no"))
	}
	if message.Action != selectionActionGrab {
		return mq.DeadLetter(fmt.Errorf("unsupported selection action: %s", message.Action))
	}

	var (
		finalStatus        string
		finalFailReason    string
		finalSelectedCount int
		finalCreditUsed    int
		notificationSpec   *StudentNotificationSpec
	)

	err := repository.DB().Transaction(func(tx *gorm.DB) error {
		// 先锁住请求记录，确保同一个 request_no 不会被重复处理。
		var request model.SelectionRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_no = ?", message.RequestNo).
			First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 请求记录都不存在，说明这条消息和数据库里的真实请求已经脱节了。
				// 这种情况继续重试通常没有意义，更适合直接进死信队列，保留样本方便排查。
				return mq.DeadLetter(fmt.Errorf("selection request not found: %s", message.RequestNo))
			}

			return err
		}

		// 幂等性处理：
		// 如果这条请求已经不是 pending，说明之前处理过了，当前这次重复消费直接跳过即可。
		if request.Status != selectionStatusPending {
			return nil
		}

		selectedCount, creditUsed, producedNotificationSpec, businessErr := applySelectCourseTx(tx, request.StudentID, request.CourseID, request.RequestNo)
		if businessErr != nil {
			// 业务错误说明“请求本身不应该成功”，
			// 所以把请求状态改成 failed，并把原因写进去。
			finalStatus = selectionStatusFailed
			finalFailReason = businessErr.Error()
			return updateSelectionRequestStatusTx(tx, &request, selectionStatusFailed, finalFailReason)
		}

		// 真正落库成功后，再把请求状态改成 success。
		finalStatus = selectionStatusSuccess
		finalSelectedCount = selectedCount
		finalCreditUsed = creditUsed
		notificationSpec = producedNotificationSpec
		return updateSelectionRequestStatusTx(tx, &request, selectionStatusSuccess, "")
	})
	if err != nil {
		// 这里只处理“数据库事务本身失败”的情况。
		// 这时 selection_requests 很可能还停留在 pending，需要在事务外单独标失败。
		failReason := fmt.Sprintf("selection consume failed: %v", err)
		if markErr := failSelectionRequest(message.RequestNo, failReason); markErr != nil {
			return fmt.Errorf("consume transaction failed: %w; mark request failed also failed: %v", err, markErr)
		}
		if cleanupErr := cleanupFailedSelection(ctx, message.StudentID, message.CourseID); cleanupErr != nil {
			return fmt.Errorf("consume transaction failed: %w; cleanup failed: %v", err, cleanupErr)
		}
		return nil
	}

	// 如果这次消费被判定为 failed，也要把 Redis 预扣减回退掉。
	if finalStatus == selectionStatusFailed {
		if cleanupErr := cleanupFailedSelection(ctx, message.StudentID, message.CourseID); cleanupErr != nil {
			return cleanupErr
		}
		_ = finalSelectedCount
		_ = finalCreditUsed
		_ = finalFailReason
	}

	if finalStatus == selectionStatusSuccess {
		publishStudentNotificationBestEffort(ctx, notificationSpec)
		_ = finalSelectedCount
		_ = finalCreditUsed
	}

	return nil
}

// tryFailTimedOutRequest 尝试把一条“超时的 pending 请求”收口成 failed。
//
// 返回值 changed 的含义是：
// - true：这次真的把请求从 pending 改成了 failed
// - false：这次什么都没改（例如请求没超时、已经成功、已经失败）
func (s *SelectionAsyncService) tryFailTimedOutRequest(ctx context.Context, studentID uint64, requestNo string) (bool, error) {
	deadline := time.Now().Add(-selectionPendingTimeout)

	var (
		courseID uint64
		changed  bool
	)

	err := repository.DB().Transaction(func(tx *gorm.DB) error {
		// 先把请求记录加锁，这样如果消费者正在处理同一条请求，
		// 两边不会同时改状态。
		var request model.SelectionRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_no = ? AND student_id = ?", requestNo, studentID).
			First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSelectionRequestNotFound
			}

			return err
		}

		// 只有“仍然是 pending 且确实超时了”的请求，才需要收口。
		if request.Status != selectionStatusPending {
			return nil
		}
		if !request.UpdatedAt.Before(deadline) {
			return nil
		}

		if err := updateSelectionRequestStatusTx(tx, &request, selectionStatusFailed, selectionFailReasonTimeout); err != nil {
			return err
		}

		courseID = request.CourseID
		changed = true
		return nil
	})
	if err != nil {
		return false, err
	}

	// 如果这次真的把请求改成了 failed，
	// 那 Redis 里的预扣减也要同步回退掉。
	if changed {
		if cleanupErr := cleanupFailedSelection(ctx, studentID, courseID); cleanupErr != nil {
			return false, cleanupErr
		}
	}

	return changed, nil
}

// applySelectCourseTx 把“真正的抢课 MySQL 事务逻辑”抽成一个可复用方法。
//
// 这样做的目的，是避免同步版、异步版各维护一份几乎一样的 MySQL 落库逻辑。
//
// 这里不负责：
// - Redis 预校验
// - 写 selection_requests
//
// 它只负责：
// - 学生/课程校验
// - enrollment 更新
// - selected_count 更新
// - credit_used 更新
// - 成功通知描述生成
func applySelectCourseTx(tx *gorm.DB, studentID, courseID uint64, requestNo string) (int, int, *StudentNotificationSpec, error) {
	var student model.Student
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&student, studentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, nil, ErrStudentNotFound
		}
		return 0, 0, nil, err
	}
	if student.Status != 1 {
		return 0, 0, nil, ErrUserDisabled
	}

	var course model.Course
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, nil, ErrCourseNotFound
		}
		return 0, 0, nil, err
	}
	if course.Status != 1 {
		return 0, 0, nil, ErrCourseClosed
	}
	if course.SelectedCount >= course.Capacity {
		return 0, 0, nil, ErrCourseFull
	}

	var enrollment model.Enrollment
	enrollmentErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("student_id = ? AND course_id = ?", studentID, courseID).
		First(&enrollment).Error
	if enrollmentErr == nil && enrollment.Status == 1 {
		return 0, 0, nil, ErrCourseAlreadySelected
	}
	if enrollmentErr != nil && !errors.Is(enrollmentErr, gorm.ErrRecordNotFound) {
		return 0, 0, nil, enrollmentErr
	}

	var conflictCount int64
	if err := tx.Model(&model.Enrollment{}).
		Joins("JOIN courses ON courses.id = enrollments.course_id").
		Where("enrollments.student_id = ? AND enrollments.status = 1 AND courses.time_slot = ?", studentID, course.TimeSlot).
		Count(&conflictCount).Error; err != nil {
		return 0, 0, nil, err
	}
	if conflictCount > 0 {
		return 0, 0, nil, ErrSelectionTimeConflict
	}

	newCreditUsed := student.CreditUsed + int(course.Credit)
	if newCreditUsed > student.CreditLimit {
		return 0, 0, nil, ErrInsufficientCredits
	}

	newSelectedCount := course.SelectedCount + 1
	if err := tx.Model(&course).Update("selected_count", newSelectedCount).Error; err != nil {
		return 0, 0, nil, err
	}
	if err := tx.Model(&student).Update("credit_used", newCreditUsed).Error; err != nil {
		return 0, 0, nil, err
	}

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
			return 0, 0, nil, err
		}
	} else {
		if err := tx.Model(&enrollment).Updates(map[string]any{
			"status":      1,
			"selected_at": now,
			"dropped_at":  nil,
		}).Error; err != nil {
			return 0, 0, nil, err
		}
	}

	relatedCourseID := course.ID
	notificationSpec := newStudentNotificationSpec(
		newNotificationBizKey("course_select_success", requestNo),
		studentID,
		"course_select_success",
		"选课成功",
		fmt.Sprintf("你已成功选上课程《%s》。", course.CourseName),
		&relatedCourseID,
		nil,
	)

	return newSelectedCount, newCreditUsed, notificationSpec, nil
}

// updateSelectionRequestStatusTx 在事务里更新一条请求记录的状态。
func updateSelectionRequestStatusTx(tx *gorm.DB, request *model.SelectionRequest, status, failReason string) error {
	updates := map[string]any{
		"status":      status,
		"fail_reason": failReason,
	}

	if err := tx.Model(request).Updates(updates).Error; err != nil {
		return err
	}

	request.Status = status
	request.FailReason = failReason
	return nil
}

// failSelectionRequest 在事务外把请求标记成 failed。
//
// 它主要用于：
// - MQ 发布失败
// - 消费者事务直接报错，来不及在事务里更新状态
func failSelectionRequest(requestNo, failReason string) error {
	return repository.DB().
		Model(&model.SelectionRequest{}).
		Where("request_no = ?", requestNo).
		Updates(map[string]any{
			"status":      selectionStatusFailed,
			"fail_reason": failReason,
		}).Error
}

// cleanupFailedSelection 统一处理“异步抢课最终失败”后的 Redis 收尾逻辑。
func cleanupFailedSelection(ctx context.Context, studentID, courseID uint64) error {
	if rollbackErr := cache.CompensateReservedSelection(ctx, studentID, courseID); rollbackErr != nil {
		if recoverErr := resetSelectionCaches(ctx, studentID, courseID); recoverErr != nil {
			return fmt.Errorf("redis compensation failed: %w; cache reset failed: %v", rollbackErr, recoverErr)
		}
	}

	return nil
}
