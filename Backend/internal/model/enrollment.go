package model

import "time"

// Enrollment 对应选课记录表。
//
// 这张表描述的是：
// “某个学生”和“某门课程”之间是否存在选课关系。
//
// 它是抢课业务的核心关系表。
type Enrollment struct {
	BaseModel
	StudentID  uint64     `gorm:"column:student_id;not null;uniqueIndex:uk_student_course,priority:1;index:idx_enrollments_student_status,priority:1;comment:学生ID" json:"student_id"`                            // 选课的学生
	CourseID   uint64     `gorm:"column:course_id;not null;uniqueIndex:uk_student_course,priority:2;index:idx_enrollments_course_status,priority:1;comment:课程ID" json:"course_id"`                               // 被选择的课程
	Status     int8       `gorm:"column:status;type:tinyint;not null;default:1;index:idx_enrollments_student_status,priority:2;index:idx_enrollments_course_status,priority:2;comment:状态 1已选 0已退" json:"status"` // 当前记录是生效选课还是已退课
	SelectedAt time.Time  `gorm:"column:selected_at;not null;comment:选课时间" json:"selected_at"`                                                                                                                   // 实际选课时间
	DroppedAt  *time.Time `gorm:"column:dropped_at;comment:退课时间" json:"dropped_at"`                                                                                                                              // 如果退课了，这里记录退课时间；没退课时是 NULL
}

// TableName 明确指定表名。
func (Enrollment) TableName() string {
	return "enrollments"
}
