package service

import (
	"context"
	"fmt"

	"choose-course-backend/internal/mq"
	"choose-course-backend/internal/pkg/logger"
	"choose-course-backend/internal/pkg/requestid"
)

const (
	// RecipientTypeStudent 表示通知接收人是学生。
	RecipientTypeStudent int8 = 1
	// RecipientTypeAdmin 表示通知接收人是管理员。
	RecipientTypeAdmin int8 = 2
)

// StudentNotificationSpec 是“事务成功后待异步发送”的学生通知描述。
//
// 业务事务里只负责把核心数据落到 MySQL；
// 等事务提交成功后，再把这份通知描述投递到 RabbitMQ，由通知消费者异步写入 notifications 表。
type StudentNotificationSpec struct {
	BizKey           string
	StudentID        uint64
	BizType          string
	Title            string
	Content          string
	RelatedCourseID  *uint64
	RelatedCommentID *uint64
}

// newStudentNotificationSpec 创建一份学生通知描述。
func newStudentNotificationSpec(
	bizKey string,
	studentID uint64,
	bizType, title, content string,
	relatedCourseID, relatedCommentID *uint64,
) *StudentNotificationSpec {
	return &StudentNotificationSpec{
		BizKey:           bizKey,
		StudentID:        studentID,
		BizType:          bizType,
		Title:            title,
		Content:          content,
		RelatedCourseID:  relatedCourseID,
		RelatedCommentID: relatedCommentID,
	}
}

// newNotificationBizKey 生成一条通知的幂等键。
//
// 如果调用方已经有天然唯一的业务标识（例如 request_no、comment_id），
// 建议直接传进来；否则这里会自动生成一段请求风格的唯一串。
func newNotificationBizKey(prefix, unique string) string {
	if unique == "" {
		unique = requestid.Generate()
	}

	return fmt.Sprintf("%s:%s", prefix, unique)
}

// publishStudentNotificationBestEffort 在事务成功后，把学生通知异步投递到 RabbitMQ。
//
// 这里明确采用 best-effort 策略：
// - 主事务已经成功，不再因为通知投递失败而回滚主业务结果
// - 通知投递失败只记录日志，留给后续排查或补偿
func publishStudentNotificationBestEffort(ctx context.Context, spec *StudentNotificationSpec) {
	if spec == nil {
		return
	}

	message := mq.NewNotificationMessage(
		spec.BizKey,
		RecipientTypeStudent,
		spec.StudentID,
		spec.BizType,
		spec.Title,
		spec.Content,
		spec.RelatedCourseID,
		spec.RelatedCommentID,
	)

	if err := mq.PublishNotification(ctx, message); err != nil {
		logPostCommitSideEffectFailure(
			"post-commit notification publish failed",
			err,
			logger.String("biz_key", spec.BizKey),
			logger.String("biz_type", spec.BizType),
			logger.Any("student_id", spec.StudentID),
		)
	}
}
