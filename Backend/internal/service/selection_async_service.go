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
	"choose-course-backend/internal/pkg/logger"
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

	selectionPendingStateTTL       = 24 * time.Hour
	selectionFinalStateTTL         = 5 * time.Minute
	selectionPendingSweepBatchSize = 200
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
// 1. 先用 Redis 做资格预校验和预扣减
// 2. 生成 request_no
// 3. 在 Redis 写一条短期 pending 状态
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

	requestState := cache.SelectionRequestState{
		RequestNo:  requestNo,
		StudentID:  studentID,
		CourseID:   courseID,
		Action:     selectionActionGrab,
		Status:     selectionStatusPending,
		FailReason: "",
	}
	if err := cache.StorePendingSelectionRequestState(ctx, requestState, selectionPendingTimeout, selectionPendingStateTTL); err != nil {
		if rollbackErr := cache.CompensateReservedSelection(ctx, studentID, courseID); rollbackErr != nil {
			if recoverErr := resetSelectionCaches(ctx, studentID, courseID); recoverErr != nil {
				return nil, fmt.Errorf("store pending request state failed: %w; redis compensation failed: %v; cache reset failed: %v", err, rollbackErr, recoverErr)
			}
		}
		return nil, err
	}

	message := mq.NewSelectionGrabMessage(requestNo, studentID, courseID)
	if err := mq.PublishSelectionGrab(ctx, message); err != nil {
		// 发布失败时，这次异步请求不算真正受理成功。
		// 所以不写 MySQL，只清理 Redis 状态并回退 Redis 预扣减。
		cleanupStateErr := cache.DeleteSelectionRequestState(ctx, requestNo)
		if rollbackErr := cache.CompensateReservedSelection(ctx, studentID, courseID); rollbackErr != nil {
			if recoverErr := resetSelectionCaches(ctx, studentID, courseID); recoverErr != nil {
				return nil, fmt.Errorf("publish selection message failed: %w; redis compensation failed: %v; cache reset failed: %v", err, rollbackErr, recoverErr)
			}
		}
		if cleanupStateErr != nil {
			return nil, fmt.Errorf("publish selection message failed: %w; cleanup request state failed: %v", err, cleanupStateErr)
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

	ctx := context.Background()

	if state, err := cache.GetSelectionRequestState(ctx, trimmedRequestNo); err != nil {
		return nil, err
	} else if state != nil {
		if state.StudentID != studentID {
			return nil, ErrSelectionRequestNotFound
		}

		if state.Status == selectionStatusPending && state.UpdatedAt.Before(time.Now().Add(-selectionPendingTimeout)) {
			if _, err := s.tryFailTimedOutRequest(ctx, studentID, trimmedRequestNo); err != nil {
				return nil, err
			}
		} else {
			return selectionRequestResultFromState(state), nil
		}
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
	candidates, err := cache.DuePendingSelectionRequestStates(ctx, time.Now(), selectionPendingSweepBatchSize)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, candidate := range candidates {
		if candidate.Status != selectionStatusPending {
			_ = cache.StoreFinalSelectionRequestState(ctx, candidate, selectionFinalStateTTL)
			continue
		}

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
// 1. 在消费者事务里创建幂等占位
// 2. 执行最终 MySQL 落库
// 3. 成功则提交 success 终态
// 4. 业务失败则提交 failed 终态，并做 Redis 补偿
// 5. 系统失败则返回错误，让 RabbitMQ 重试
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

	createdAt := time.Now()
	if message.CreatedAt > 0 {
		createdAt = time.Unix(message.CreatedAt, 0)
	}

	messageState := cache.SelectionRequestState{
		RequestNo: message.RequestNo,
		StudentID: message.StudentID,
		CourseID:  message.CourseID,
		Action:    message.Action,
		Status:    selectionStatusPending,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}

	var finalState *cache.SelectionRequestState
	var notificationSpec *StudentNotificationSpec

	err := repository.DB().Transaction(func(tx *gorm.DB) error {
		request, shouldProcess, err := createProcessingSelectionRequestTx(tx, messageState)
		if err != nil {
			return err
		}

		if !shouldProcess {
			state := selectionRequestStateFromModel(request)
			finalState = &state
			return nil
		}

		_, _, producedNotificationSpec, businessErr := applySelectCourseTx(tx, request.StudentID, request.CourseID, request.RequestNo)
		if businessErr != nil {
			// 业务错误说明“请求本身不应该成功”，
			// 所以把请求状态改成 failed，并把原因写进去。
			if err := updateSelectionRequestStatusTx(tx, request, selectionStatusFailed, businessErr.Error()); err != nil {
				return err
			}

			state := selectionRequestStateFromModel(request)
			finalState = &state
			return nil
		}

		// 真正落库成功后，再把请求状态改成 success。
		notificationSpec = producedNotificationSpec
		if err := updateSelectionRequestStatusTx(tx, request, selectionStatusSuccess, ""); err != nil {
			return err
		}

		state := selectionRequestStateFromModel(request)
		finalState = &state
		return nil
	})
	if err != nil {
		// 数据库或临时系统错误不再被直接收口成 failed。
		// 返回错误交给 RabbitMQ 重试，Redis 预扣减继续保留为 pending。
		return err
	}

	if finalState == nil {
		return nil
	}

	// 如果这次消费被判定为 failed，也要把 Redis 预扣减回退掉。
	if finalState.Status == selectionStatusFailed {
		if cleanupErr := cleanupFailedSelection(ctx, finalState.StudentID, finalState.CourseID); cleanupErr != nil {
			return cleanupErr
		}
	}

	if err := cache.StoreFinalSelectionRequestState(ctx, *finalState, selectionFinalStateTTL); err != nil {
		logPostCommitCacheSyncFailure(
			"store final selection request state failed",
			err,
			logger.Any("student_id", finalState.StudentID),
			logger.Any("course_id", finalState.CourseID),
			logger.String("request_no", finalState.RequestNo),
		)
	}

	if finalState.Status == selectionStatusSuccess {
		publishStudentNotificationBestEffort(ctx, notificationSpec)
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

	state, err := cache.GetSelectionRequestState(ctx, requestNo)
	if err != nil {
		return false, err
	}
	if state == nil {
		return false, nil
	}
	if state.StudentID != studentID {
		return false, ErrSelectionRequestNotFound
	}

	// 只有“仍然是 pending 且确实超时了”的请求，才需要收口。
	if state.Status != selectionStatusPending {
		if err := cache.StoreFinalSelectionRequestState(ctx, *state, selectionFinalStateTTL); err != nil {
			return false, err
		}
		return false, nil
	}
	if !state.UpdatedAt.Before(deadline) {
		return false, nil
	}

	finalRequest, created, err := createFinalFailedSelectionRequest(*state, selectionFailReasonTimeout)
	if err != nil {
		return false, err
	}

	finalState := selectionRequestStateFromModel(finalRequest)

	if created || finalRequest.Status == selectionStatusFailed {
		if cleanupErr := cleanupFailedSelection(ctx, state.StudentID, state.CourseID); cleanupErr != nil {
			return false, cleanupErr
		}
	}

	if err := cache.StoreFinalSelectionRequestState(ctx, finalState, selectionFinalStateTTL); err != nil {
		return false, err
	}

	return created, nil
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
	now := time.Now()
	updates := map[string]any{
		"status":      status,
		"fail_reason": failReason,
		"updated_at":  now,
	}

	if err := tx.Model(request).Updates(updates).Error; err != nil {
		return err
	}

	request.Status = status
	request.FailReason = failReason
	request.UpdatedAt = now
	return nil
}

// createProcessingSelectionRequestTx creates a transaction-local pending row as
// the idempotency gate. The row is always updated to a final status before the
// transaction commits, so MySQL does not expose entrance-side pending requests.
func createProcessingSelectionRequestTx(tx *gorm.DB, state cache.SelectionRequestState) (*model.SelectionRequest, bool, error) {
	request := selectionRequestModelFromState(state, selectionStatusPending, "")
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "request_no"}},
		DoNothing: true,
	}).Create(&request)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected > 0 {
		return &request, true, nil
	}

	var existing model.SelectionRequest
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("request_no = ?", state.RequestNo).
		First(&existing).Error; err != nil {
		return nil, false, err
	}

	return &existing, existing.Status == selectionStatusPending, nil
}

func createFinalFailedSelectionRequest(state cache.SelectionRequestState, failReason string) (*model.SelectionRequest, bool, error) {
	request := selectionRequestModelFromState(state, selectionStatusFailed, failReason)
	result := repository.DB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "request_no"}},
		DoNothing: true,
	}).Create(&request)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected > 0 {
		return &request, true, nil
	}

	var existing model.SelectionRequest
	if err := repository.DB().
		Where("request_no = ? AND student_id = ?", state.RequestNo, state.StudentID).
		First(&existing).Error; err != nil {
		return nil, false, err
	}

	if existing.Status != selectionStatusPending {
		return &existing, false, nil
	}

	var changed bool
	err := repository.DB().Transaction(func(tx *gorm.DB) error {
		var locked model.SelectionRequest
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_no = ? AND student_id = ?", state.RequestNo, state.StudentID).
			First(&locked).Error; err != nil {
			return err
		}

		if locked.Status != selectionStatusPending {
			existing = locked
			return nil
		}

		if err := updateSelectionRequestStatusTx(tx, &locked, selectionStatusFailed, failReason); err != nil {
			return err
		}

		existing = locked
		changed = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	return &existing, changed, nil
}

func selectionRequestModelFromState(state cache.SelectionRequestState, status, failReason string) model.SelectionRequest {
	request := model.SelectionRequest{
		RequestNo:  state.RequestNo,
		StudentID:  state.StudentID,
		CourseID:   state.CourseID,
		Action:     state.Action,
		Status:     status,
		FailReason: failReason,
	}
	if !state.CreatedAt.IsZero() {
		request.CreatedAt = state.CreatedAt
		request.UpdatedAt = state.CreatedAt
	}

	return request
}

func selectionRequestStateFromModel(request *model.SelectionRequest) cache.SelectionRequestState {
	return cache.SelectionRequestState{
		RequestNo:  request.RequestNo,
		StudentID:  request.StudentID,
		CourseID:   request.CourseID,
		Action:     request.Action,
		Status:     request.Status,
		FailReason: request.FailReason,
		CreatedAt:  request.CreatedAt,
		UpdatedAt:  request.UpdatedAt,
	}
}

func selectionRequestResultFromState(state *cache.SelectionRequestState) *SelectionRequestResult {
	return &SelectionRequestResult{
		RequestNo:  state.RequestNo,
		StudentID:  state.StudentID,
		CourseID:   state.CourseID,
		Action:     state.Action,
		Status:     state.Status,
		FailReason: state.FailReason,
		CreatedAt:  state.CreatedAt,
		UpdatedAt:  state.UpdatedAt,
	}
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
