package service

import (
	"errors"
	"time"

	"choose-course-backend/internal/model"
	"choose-course-backend/internal/repository"
	"gorm.io/gorm"
)

var (
	// ErrNotificationNotFound 表示通知不存在，或者不属于当前学生。
	ErrNotificationNotFound = errors.New("notification not found")
)

// NotificationListInput 是消息列表分页参数。
type NotificationListInput struct {
	Page     int
	PageSize int
	IsRead   *int8
}

// NotificationItem 是单条通知返回结构。
type NotificationItem struct {
	ID               uint64    `json:"id"`
	BizType          string    `json:"biz_type"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	RelatedCourseID  *uint64   `json:"related_course_id"`
	RelatedCommentID *uint64   `json:"related_comment_id"`
	IsRead           int8      `json:"is_read"`
	CreatedAt        time.Time `json:"created_at"`
}

// NotificationListResult 是消息列表返回结构。
type NotificationListResult struct {
	List     []NotificationItem `json:"list"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

// StudentNotificationService 负责学生消息中心。
type StudentNotificationService struct{}

// NewStudentNotificationService 创建学生通知服务。
func NewStudentNotificationService() *StudentNotificationService {
	return &StudentNotificationService{}
}

// ListNotifications 分页查询学生自己的消息列表。
//
// 这个方法和 handler 的分工不一样：
// - handler 负责从 HTTP 请求里取参数
// - service 负责真正组织数据库查询和返回业务结果
//
// 它整体做的事情可以拆成 4 步：
// 1. 规范化分页参数
// 2. 先构造“当前学生自己的通知”这个基础查询条件
// 3. 根据是否传了 is_read，决定要不要加“已读/未读”过滤
// 4. 先查总数，再查当前页数据，最后转换成前端返回结构
func (s *StudentNotificationService) ListNotifications(studentID uint64, input NotificationListInput) (*NotificationListResult, error) {
	// 对分页参数做兜底，避免 page/pageSize 传入非法值。
	page := input.Page
	if page <= 0 {
		page = 1
	}

	// pageSize 默认 10，最大限制 100，避免一次查太多数据。
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 先构造基础查询：
	// 这里只查“当前学生自己的通知”。
	// recipient_type = RecipientTypeStudent 表示接收人类型必须是学生，
	// recipient_id = studentID 表示接收人必须是当前登录学生本人。
	db := repository.DB().Model(&model.Notification{}).
		Where("recipient_type = ? AND recipient_id = ?", RecipientTypeStudent, studentID)

	// is_read 是一个可选过滤条件：
	// - nil：不区分已读未读，全部都查
	// - 0：只查未读
	// - 1：只查已读
	if input.IsRead != nil {
		db = db.Where("is_read = ?", *input.IsRead)
	}

	// total 是满足条件的通知总数，不是当前页的条数。
	// 前端做分页时需要它来计算总页数。
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	// 再查询当前页数据。
	// Order("id DESC") 表示按最新通知优先返回。
	var notifications []model.Notification
	if err := db.
	            Order("id DESC").
	            Offset((page - 1) * pageSize).
				Limit(pageSize).
				Find(&notifications).Error; err != nil {
		return nil, err
	}

	// 把数据库模型转换成接口返回结构。
	// 这里不直接把 model.Notification 原样返回，是为了控制给前端暴露哪些字段。
	list := make([]NotificationItem, 0, len(notifications))
	for _, notification := range notifications {
		list = append(list, NotificationItem{
			ID:               notification.ID,
			BizType:          notification.BizType,
			Title:            notification.Title,
			Content:          notification.Content,
			RelatedCourseID:  notification.RelatedCourseID,
			RelatedCommentID: notification.RelatedCommentID,
			IsRead:           notification.IsRead,
			CreatedAt:        notification.CreatedAt,
		})
	}

	// 最后把“当前页数据 + 总数 + 分页信息”一起返回给调用方。
	return &NotificationListResult{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// MarkRead 把一条消息标记为已读。
func (s *StudentNotificationService) MarkRead(studentID, notificationID uint64) (*NotificationItem, error) {
	var notification model.Notification
	if err := repository.DB().
		Where("id = ? AND recipient_type = ? AND recipient_id = ?", notificationID, RecipientTypeStudent, studentID).
		First(&notification).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotificationNotFound
		}

		return nil, err
	}

	if notification.IsRead != 1 {
		if err := repository.DB().Model(&notification).Update("is_read", 1).Error; err != nil {
			return nil, err
		}
		notification.IsRead = 1
	}

	return &NotificationItem{
		ID:               notification.ID,
		BizType:          notification.BizType,
		Title:            notification.Title,
		Content:          notification.Content,
		RelatedCourseID:  notification.RelatedCourseID,
		RelatedCommentID: notification.RelatedCommentID,
		IsRead:           notification.IsRead,
		CreatedAt:        notification.CreatedAt,
	}, nil
}
