package model

// CourseLike 对应课程点赞表。
//
// 这张表非常简单，它只回答一个问题：
// “这个学生有没有给这门课点过赞？”
type CourseLike struct {
	BaseModel
	StudentID uint64 `gorm:"column:student_id;not null;uniqueIndex:uk_like_student_course,priority:1;comment:学生ID" json:"student_id"`                                // 点赞的学生
	CourseID  uint64 `gorm:"column:course_id;not null;uniqueIndex:uk_like_student_course,priority:2;index:idx_course_likes_course_id;comment:课程ID" json:"course_id"` // 被点赞的课程
}

// TableName 明确指定表名。
func (CourseLike) TableName() string {
	return "course_likes"
}
