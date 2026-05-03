package model

// CourseComment 对应课程评论表。
//
// 学生给课程留言时，就会写一条评论记录到这里。
type CourseComment struct {
	BaseModel
	StudentID uint64 `gorm:"column:student_id;not null;index:idx_course_comments_student_created,priority:1;comment:学生ID" json:"student_id"` // 评论是谁发的
	CourseID  uint64 `gorm:"column:course_id;not null;index:idx_course_comments_course_created,priority:1;comment:课程ID" json:"course_id"`    // 评论属于哪门课
	Content   string `gorm:"column:content;size:500;not null;comment:评论内容" json:"content"`                                                   // 评论正文，当前长度限制 500
	Status    int8   `gorm:"column:status;type:tinyint;not null;default:1;comment:状态 1正常 0已删除" json:"status"`                                // 逻辑状态，用来表示评论是否被删除
}

// TableName 明确指定表名。
func (CourseComment) TableName() string {
	return "course_comments"
}
