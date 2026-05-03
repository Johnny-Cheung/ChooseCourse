package service

import (
	"context"
	"fmt"
	"strings"

	"choose-course-backend/internal/model"
	"choose-course-backend/internal/mq"
	"choose-course-backend/internal/repository"
)

// NotificationAsyncService 负责消费异步通知消息，并落库到 notifications 表。
type NotificationAsyncService struct{}

// NewNotificationAsyncService 创建通知异步服务实例。
func NewNotificationAsyncService() *NotificationAsyncService {
	return &NotificationAsyncService{}
}

// HandleMessage 消费一条通知消息。
func (s *NotificationAsyncService) HandleMessage(_ context.Context, message mq.NotificationMessage) error {
	if err := validateNotificationMessage(message); err != nil {
		return mq.DeadLetter(err)
	}

	notification := model.Notification{
		BizKey:           message.BizKey,
		RecipientType:    message.RecipientType,
		RecipientID:      message.RecipientID,
		BizType:          message.BizType,
		Title:            message.Title,
		Content:          message.Content,
		RelatedCourseID:  message.RelatedCourseID,
		RelatedCommentID: message.RelatedCommentID,
		IsRead:           0,
	}

	if err := repository.DB().Create(&notification).Error; err != nil {
		if isDuplicateEntry(err) {
			return nil
		}
		return err
	}

	return nil
}

// validateNotificationMessage 做最小但必要的通知消息校验。
func validateNotificationMessage(message mq.NotificationMessage) error {
	if strings.TrimSpace(message.BizKey) == "" {
		return fmt.Errorf("notification message missing biz_key")
	}
	if message.RecipientType != RecipientTypeStudent && message.RecipientType != RecipientTypeAdmin {
		return fmt.Errorf("notification message invalid recipient_type: %d", message.RecipientType)
	}
	if message.RecipientID == 0 {
		return fmt.Errorf("notification message missing recipient_id")
	}
	if strings.TrimSpace(message.BizType) == "" {
		return fmt.Errorf("notification message missing biz_type")
	}
	if strings.TrimSpace(message.Title) == "" {
		return fmt.Errorf("notification message missing title")
	}
	if strings.TrimSpace(message.Content) == "" {
		return fmt.Errorf("notification message missing content")
	}

	return nil
}
