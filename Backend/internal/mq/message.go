package mq

import "time"

// SelectionMessage 是投递到 RabbitMQ 的抢课消息体。
//
// 这里带上 request_no 的原因是：
// M7 里真正的“请求状态真相”会落在 selection_requests 表里，
// 消费者拿到消息后，需要先根据 request_no 找到对应的请求记录，再决定是否继续处理。
type SelectionMessage struct {
	RequestNo string `json:"request_no"` // 本次异步抢课请求编号
	StudentID uint64 `json:"student_id"` // 发起请求的学生 ID
	CourseID  uint64 `json:"course_id"`  // 目标课程 ID
	Action    string `json:"action"`     // 动作类型，当前 M7 第一版只会用到 grab
	CreatedAt int64  `json:"created_at"` // 消息创建时间戳，方便后续排查和日志追踪
}

// NewSelectionGrabMessage 创建一条“抢课消息”。
func NewSelectionGrabMessage(requestNo string, studentID, courseID uint64) SelectionMessage {
	return SelectionMessage{
		RequestNo: requestNo,
		StudentID: studentID,
		CourseID:  courseID,
		Action:    "grab",
		CreatedAt: time.Now().Unix(),
	}
}

// NotificationMessage 是投递到 RabbitMQ 的通知消息体。
//
// 它承载的是“主业务成功后，需要异步写入消息中心”的副作用事件。
// BizKey 用作通知落库的幂等键，避免消息重复消费时重复插入 notifications。
type NotificationMessage struct {
	BizKey           string  `json:"biz_key"`
	RecipientType    int8    `json:"recipient_type"`
	RecipientID      uint64  `json:"recipient_id"`
	BizType          string  `json:"biz_type"`
	Title            string  `json:"title"`
	Content          string  `json:"content"`
	RelatedCourseID  *uint64 `json:"related_course_id,omitempty"`
	RelatedCommentID *uint64 `json:"related_comment_id,omitempty"`
	CreatedAt        int64   `json:"created_at"`
}

// NewNotificationMessage 创建一条通知消息。
func NewNotificationMessage(
	bizKey string,
	recipientType int8,
	recipientID uint64,
	bizType, title, content string,
	relatedCourseID, relatedCommentID *uint64,
) NotificationMessage {
	return NotificationMessage{
		BizKey:           bizKey,
		RecipientType:    recipientType,
		RecipientID:      recipientID,
		BizType:          bizType,
		Title:            title,
		Content:          content,
		RelatedCourseID:  relatedCourseID,
		RelatedCommentID: relatedCommentID,
		CreatedAt:        time.Now().Unix(),
	}
}
