package model

// SelectionRequest 对应抢课请求状态表。
//
// 这张表不是最终的“选课关系表”，而是“请求处理记录表”。
// 它主要为后续高并发异步抢课做准备。
//
// 你可以把它理解成：
// 学生发起了一次抢课请求，系统要记下来：
// - 谁发的
// - 抢的是哪门课
// - 当前处理成功没有
// - 如果失败，失败原因是什么
type SelectionRequest struct {
	BaseModel
	RequestNo  string `gorm:"column:request_no;size:64;not null;uniqueIndex:uk_request_no;comment:请求号" json:"request_no"`                        // 业务请求号，用于幂等控制和状态查询
	StudentID  uint64 `gorm:"column:student_id;not null;index:idx_selection_requests_student_created,priority:1;comment:学生ID" json:"student_id"` // 发起请求的学生
	CourseID   uint64 `gorm:"column:course_id;not null;index:idx_selection_requests_course_created,priority:1;comment:课程ID" json:"course_id"`    // 目标课程
	Action     string `gorm:"column:action;size:16;not null;comment:动作 grab/drop" json:"action"`                                                 // 本次请求是抢课还是退课
	Status     string `gorm:"column:status;size:16;not null;comment:状态 pending/success/failed" json:"status"`                                    // 当前处理状态
	FailReason string `gorm:"column:fail_reason;size:255;comment:失败原因" json:"fail_reason"`                                                       // 如果失败，把失败原因写在这里
}

// TableName 明确指定表名。
func (SelectionRequest) TableName() string {
	return "selection_requests"
}
