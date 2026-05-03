package model

// Course 对应课程表。
//
// 这是抢课系统里最核心的业务表之一。
// 一门课是否能被选、还能选几个人、是否会冲突，很多判断都来自这里。
type Course struct {
	SoftDeleteModel
	CourseName    string `gorm:"column:course_name;size:128;not null;index:idx_courses_course_name;comment:课程名称" json:"course_name"`                            // 课程名称，支持按名字搜索
	TeacherName   string `gorm:"column:teacher_name;size:64;not null;index:idx_courses_teacher_name;comment:教师姓名" json:"teacher_name"`                          // 任课教师姓名，支持按老师筛选
	Capacity      int    `gorm:"column:capacity;not null;comment:课程容量" json:"capacity"`                                                                         // 这门课总共能容纳多少人
	SelectedCount int    `gorm:"column:selected_count;not null;default:0;comment:已选人数" json:"selected_count"`                                                   // 当前已选人数，是一个冗余但非常重要的计数字段
	TimeSlot      int8   `gorm:"column:time_slot;type:tinyint;not null;index:idx_courses_status_time_slot,priority:2;comment:时间片 0-20" json:"time_slot"`        // 上课时间片，后面要靠它判断课程冲突
	Credit        int8   `gorm:"column:credit;type:tinyint;not null;comment:学分 2/3/4" json:"credit"`                                                            // 课程学分，抢课时会消耗学生学分额度
	Status        int8   `gorm:"column:status;type:tinyint;not null;default:1;index:idx_courses_status_time_slot,priority:1;comment:状态 0不开课 1开课" json:"status"` // 当前是否开课，只有开课课程才能抢
	LikeCount     int    `gorm:"column:like_count;not null;default:0;comment:点赞数" json:"like_count"`                                                            // 点赞数，属于冗余计数，便于列表直接展示
	CommentCount  int    `gorm:"column:comment_count;not null;default:0;comment:评论数" json:"comment_count"`                                                      // 评论数，便于课程列表/详情快速展示
	Version       int    `gorm:"column:version;not null;default:1;comment:乐观锁版本号" json:"version"`                                                               // 预留给后续高并发更新时做乐观锁控制
}

// TableName 明确指定表名。
func (Course) TableName() string {
	return "courses"
}
