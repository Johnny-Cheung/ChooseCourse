package service

import (
	"errors"

	"choose-course-backend/internal/model"
	"choose-course-backend/internal/repository"
	"gorm.io/gorm"
)

// StudentCourseListInput 表示学生课程列表查询条件。
type StudentCourseListInput struct {
	Page        int
	PageSize    int
	CourseName  string
	TeacherName string
	Status      *int8
	TimeSlot    *int8
	Credit      *int8
}

// StudentCourseSummary 是课程列表里返回的单条课程概要信息。
type StudentCourseSummary struct {
	ID            uint64 `json:"id"`
	CourseName    string `json:"course_name"`
	TeacherName   string `json:"teacher_name"`
	Status        int8   `json:"status"`
	TimeSlot      int8   `json:"time_slot"`
	Credit        int8   `json:"credit"`
	Capacity      int    `json:"capacity"`
	SelectedCount int    `json:"selected_count"`
	LikeCount     int    `json:"like_count"`
	CommentCount  int    `json:"comment_count"`
}

// StudentCourseListResult 是学生课程列表接口返回结构。
type StudentCourseListResult struct {
	List     []StudentCourseSummary `json:"list"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// StudentCourseService 负责学生课程列表、搜索和详情。
type StudentCourseService struct{}

// NewStudentCourseService 创建学生课程服务。
func NewStudentCourseService() *StudentCourseService {
	return &StudentCourseService{}
}

// ListCourses 返回学生可见课程的分页列表。
func (s *StudentCourseService) ListCourses(input StudentCourseListInput) (*StudentCourseListResult, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}

	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	db := repository.DB().Model(&model.Course{})

	if input.CourseName != "" {
		db = db.Where("course_name LIKE ?", "%"+input.CourseName+"%")
	}
	if input.TeacherName != "" {
		db = db.Where("teacher_name LIKE ?", "%"+input.TeacherName+"%")
	}
	if input.Status != nil {
		db = db.Where("status = ?", *input.Status)
	}
	if input.TimeSlot != nil {
		db = db.Where("time_slot = ?", *input.TimeSlot)
	}
	if input.Credit != nil {
		db = db.Where("credit = ?", *input.Credit)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	var courses []model.Course
	if err := db.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&courses).Error; err != nil {
		return nil, err
	}

	list := make([]StudentCourseSummary, 0, len(courses))
	for _, course := range courses {
		list = append(list, StudentCourseSummary{
			ID:            course.ID,
			CourseName:    course.CourseName,
			TeacherName:   course.TeacherName,
			Status:        course.Status,
			TimeSlot:      course.TimeSlot,
			Credit:        course.Credit,
			Capacity:      course.Capacity,
			SelectedCount: course.SelectedCount,
			LikeCount:     course.LikeCount,
			CommentCount:  course.CommentCount,
		})
	}

	return &StudentCourseListResult{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetCourseByID 返回单门课程详情。
func (s *StudentCourseService) GetCourseByID(courseID uint64) (*CourseDetail, error) {
	var course model.Course
	if err := repository.DB().First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCourseNotFound
		}

		return nil, err
	}

	return toCourseDetail(course), nil
}
