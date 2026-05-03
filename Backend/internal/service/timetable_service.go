package service

import (
	"choose-course-backend/internal/repository"
	"time"
)

// TimetableItem 表示课表里的一门课。
type TimetableItem struct {
	CourseID    uint64    `json:"course_id"`
	CourseName  string    `json:"course_name"`
	TeacherName string    `json:"teacher_name"`
	Credit      int8      `json:"credit"`
	TimeSlot    int8      `json:"time_slot"`
	SelectedAt  time.Time `json:"selected_at"`
}

// TimetableResult 是课表接口的统一返回结构。
type TimetableResult struct {
	List []TimetableItem `json:"list"`
}

// TimetableService 负责学生课表查询。
type TimetableService struct{}

// NewTimetableService 创建一个课表服务实例。
func NewTimetableService() *TimetableService {
	return &TimetableService{}
}

// GetTimetable 查询当前学生的有效课表。
//
// 这里查的是：
// - 当前学生 studentID
// - 仍然有效的选课记录 enrollments.status = 1
// - 再把课程表 courses 连上，补全课程名称、老师、学分、时间片
//
// 最终结果按 time_slot 升序返回，方便前端直接展示。
func (s *TimetableService) GetTimetable(studentID uint64) (*TimetableResult, error) {
	var rows []struct {
		CourseID    uint64    `gorm:"column:course_id"`
		CourseName  string    `gorm:"column:course_name"`
		TeacherName string    `gorm:"column:teacher_name"`
		Credit      int8      `gorm:"column:credit"`
		TimeSlot    int8      `gorm:"column:time_slot"`
		SelectedAt  time.Time `gorm:"column:selected_at"`
	}

	if err := repository.DB().
		Table("enrollments").
		Select("enrollments.course_id, courses.course_name, courses.teacher_name, courses.credit, courses.time_slot, enrollments.selected_at").
		Joins("JOIN courses ON courses.id = enrollments.course_id").
		Where("enrollments.student_id = ? AND enrollments.status = 1", studentID).
		Order("courses.time_slot ASC").
		Order("enrollments.selected_at ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	list := make([]TimetableItem, 0, len(rows))
	for _, row := range rows {
		list = append(list, TimetableItem{
			CourseID:    row.CourseID,
			CourseName:  row.CourseName,
			TeacherName: row.TeacherName,
			Credit:      row.Credit,
			TimeSlot:    row.TimeSlot,
			SelectedAt:  row.SelectedAt,
		})
	}

	return &TimetableResult{List: list}, nil
}
