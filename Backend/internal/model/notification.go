package model

// Notification 对应消息通知表。
//
// 以后学生抢课成功、退课成功、评论成功等，都可以往这张表里写消息。
// 前端“消息中心”看的就是这张表。
type Notification struct {
	BaseModel
	BizKey           string  `gorm:"column:biz_key;size:128;not null;uniqueIndex:uk_notifications_biz_key;comment:通知幂等键" json:"-"`                                                       // 用于保证异步通知重复消费时不重复落库
	RecipientType    int8    `gorm:"column:recipient_type;type:tinyint;not null;index:idx_notifications_recipient_read_created,priority:1;comment:接收人类型 1学生 2管理员" json:"recipient_type"` // 这条消息发给学生还是管理员
	RecipientID      uint64  `gorm:"column:recipient_id;not null;index:idx_notifications_recipient_read_created,priority:2;comment:接收人ID" json:"recipient_id"`                           // 接收人的主键 ID
	BizType          string  `gorm:"column:biz_type;size:32;not null;comment:业务类型" json:"biz_type"`                                                                                      // 业务类型，例如 course_select_success
	Title            string  `gorm:"column:title;size:128;not null;comment:标题" json:"title"`                                                                                             // 消息标题
	Content          string  `gorm:"column:content;size:500;not null;comment:内容" json:"content"`                                                                                         // 消息正文
	RelatedCourseID  *uint64 `gorm:"column:related_course_id;comment:关联课程ID" json:"related_course_id"`                                                                                   // 如果这条消息和课程有关，这里记录课程 ID
	RelatedCommentID *uint64 `gorm:"column:related_comment_id;comment:关联评论ID" json:"related_comment_id"`                                                                                 // 如果这条消息和评论有关，这里记录评论 ID
	IsRead           int8    `gorm:"column:is_read;type:tinyint;not null;default:0;index:idx_notifications_recipient_read_created,priority:3;comment:是否已读" json:"is_read"`               // 是否已读，0 未读，1 已读
}

// TableName 明确指定表名。
func (Notification) TableName() string {
	return "notifications"
}
