package service

import (
	"context"
	"errors"

	"choose-course-backend/internal/cache"
	"choose-course-backend/internal/model"
	"choose-course-backend/internal/pkg/logger"
	"choose-course-backend/internal/repository"
	"gorm.io/gorm"
)

var (
	// ErrCourseNotFound 表示课程不存在。
	ErrCourseNotFound = errors.New("course not found")
	// ErrInvalidCourseStatus 表示课程状态不合法。
	ErrInvalidCourseStatus = errors.New("invalid course status")
	// ErrInvalidCourseCredit 表示课程学分不合法。
	ErrInvalidCourseCredit = errors.New("invalid course credit")
	// ErrInvalidCourseTimeSlot 表示课程时间片不合法。
	ErrInvalidCourseTimeSlot = errors.New("invalid course time slot")
	// ErrInvalidCourseCapacity 表示课程容量不合法。
	ErrInvalidCourseCapacity = errors.New("invalid course capacity")
	// ErrCourseCannotClose 表示已有人选课后不能下线。
	ErrCourseCannotClose = errors.New("course cannot close")
	// ErrCourseCapacityTooSmall 表示修改后的容量小于已选人数。
	ErrCourseCapacityTooSmall = errors.New("course capacity too small")
	// ErrCourseImmutableAfterSelected 表示选课后禁止修改部分关键字段。
	ErrCourseImmutableAfterSelected = errors.New("course immutable after selected")
)

// CreateCourseInput 是管理员新增课程时提交的数据。
type CreateCourseInput struct {
	CourseName  string // 课程名称
	TeacherName string // 教师姓名
	Capacity    int    // 课程容量
	TimeSlot    int8   // 时间片
	Credit      int8   // 学分
	Status      int8   // 状态
}

// UpdateCourseInput 是管理员修改课程时允许提交的数据。
// 所有字段都用指针，表示“传了才更新”。
type UpdateCourseInput struct {
	CourseName  *string
	TeacherName *string
	Capacity    *int
	TimeSlot    *int8
	Credit      *int8
	Status      *int8
}

// CourseDetail 是管理端查看课程详情时返回的数据。
type CourseDetail struct {
	ID            uint64 `json:"id"`
	CourseName    string `json:"course_name"`
	TeacherName   string `json:"teacher_name"`
	Capacity      int    `json:"capacity"`
	SelectedCount int    `json:"selected_count"`
	TimeSlot      int8   `json:"time_slot"`
	Credit        int8   `json:"credit"`
	Status        int8   `json:"status"`
	LikeCount     int    `json:"like_count"`
	CommentCount  int    `json:"comment_count"`
	Version       int    `json:"version"`
}

// ListCoursesInput 是课程列表查询条件。
type ListCoursesInput struct {
	Page        int
	PageSize    int
	CourseName  string
	TeacherName string
	Status      *int8
	TimeSlot    *int8
	Credit      *int8
}

// CourseListResult 是列表接口返回结构。
type CourseListResult struct {
	List     []CourseDetail `json:"list"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// AdminCourseService 负责管理员侧课程管理。
type AdminCourseService struct{}

// NewAdminCourseService 创建课程管理服务。
func NewAdminCourseService() *AdminCourseService {
	return &AdminCourseService{}
}

// CreateCourse 新增课程。
func (s *AdminCourseService) CreateCourse(input CreateCourseInput) (*CourseDetail, error) {
	if err := validateCourseFields(input.Capacity, input.TimeSlot, input.Credit, input.Status); err != nil {
		return nil, err
	}

	course := model.Course{
		CourseName:    input.CourseName,
		TeacherName:   input.TeacherName,
		Capacity:      input.Capacity,
		SelectedCount: 0,
		TimeSlot:      input.TimeSlot,
		Credit:        input.Credit,
		Status:        input.Status,
		LikeCount:     0,
		CommentCount:  0,
		Version:       1,
	}

	if err := repository.DB().Create(&course).Error; err != nil {
		return nil, err
	}

	if course.Status == 1 {
		if err := cache.RefreshCourseSelectionCache(context.Background(), course.ID); err != nil {
			logPostCommitCacheSyncFailure(
				"post-create course cache warm-up failed",
				err,
				logger.Any("course_id", course.ID),
			)
		}
	}

	return toCourseDetail(course), nil
}

// GetCourseByID 按课程 ID 查询课程详情。
func (s *AdminCourseService) GetCourseByID(courseID uint64) (*CourseDetail, error) {
	var course model.Course
	if err := repository.DB().First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCourseNotFound
		}

		return nil, err
	}

	return toCourseDetail(course), nil
}

// ListCourses 返回分页课程列表。
func (s *AdminCourseService) ListCourses(input ListCoursesInput) (*CourseListResult, error) {
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

	// 这些过滤条件都是可选的，传了才参与查询。
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

	list := make([]CourseDetail, 0, len(courses))
	for _, course := range courses {
		list = append(list, *toCourseDetail(course))
	}

	return &CourseListResult{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UpdateCourse 按课程 ID 修改课程。
func (s *AdminCourseService) UpdateCourse(courseID uint64, input UpdateCourseInput) (*CourseDetail, error) {
	var course model.Course
	if err := repository.DB().First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCourseNotFound
		}

		return nil, err
	}

	updates := make(map[string]any)

	if input.CourseName != nil {
		updates["course_name"] = *input.CourseName
	}
	if input.TeacherName != nil {
		updates["teacher_name"] = *input.TeacherName
	}
	if input.Capacity != nil {
		if *input.Capacity <= 0 {
			return nil, ErrInvalidCourseCapacity
		}
		if *input.Capacity < course.SelectedCount {
			return nil, ErrCourseCapacityTooSmall
		}
		updates["capacity"] = *input.Capacity
	}
	if input.Status != nil {
		if *input.Status != 0 && *input.Status != 1 {
			return nil, ErrInvalidCourseStatus
		}
		if course.SelectedCount > 0 && *input.Status == 0 {
			return nil, ErrCourseCannotClose
		}
		updates["status"] = *input.Status
	}
	if input.TimeSlot != nil {
		if *input.TimeSlot < 0 || *input.TimeSlot > 20 {
			return nil, ErrInvalidCourseTimeSlot
		}
		if course.SelectedCount > 0 && *input.TimeSlot != course.TimeSlot {
			return nil, ErrCourseImmutableAfterSelected
		}
		updates["time_slot"] = *input.TimeSlot
	}
	if input.Credit != nil {
		if *input.Credit != 2 && *input.Credit != 3 && *input.Credit != 4 {
			return nil, ErrInvalidCourseCredit
		}
		if course.SelectedCount > 0 && *input.Credit != course.Credit {
			return nil, ErrCourseImmutableAfterSelected
		}
		updates["credit"] = *input.Credit
	}

	if len(updates) > 0 {
		if err := repository.DB().Model(&course).Updates(updates).Error; err != nil {
			return nil, err
		}

		// M6 之后，抢课前置校验会依赖 Redis 里的课程缓存。
		// 所以管理员修改课程后，不能继续让旧缓存留在 Redis 里。
		//
		// 这里不选择“立刻刷新缓存”，而是采用更简单的 cache-aside 思路：
		// 1. 先把 MySQL 改对
		// 2. 再删除课程缓存
		// 3. 等下一次真正有人用到这门课时，再从 MySQL 回源重建缓存
		//
		// 这样做的好处是：
		// - 逻辑简单，适合当前这个个人项目
		// - 不会因为一次管理端修改就立刻触发额外的缓存重建
		// - 能避免“连续多次修改课程，连续多次刷新缓存”的重复工作
		if err := cache.InvalidateCourseSelectionCache(context.Background(), courseID); err != nil {
			logPostCommitCacheSyncFailure(
				"post-commit course cache invalidation failed",
				err,
				logger.Any("course_id", courseID),
			)
		}
	}

	return s.GetCourseByID(courseID)
}

// validateCourseFields 校验课程新增时的核心字段。
func validateCourseFields(capacity int, timeSlot, credit, status int8) error {
	if capacity <= 0 {
		return ErrInvalidCourseCapacity
	}
	if timeSlot < 0 || timeSlot > 20 {
		return ErrInvalidCourseTimeSlot
	}
	if credit != 2 && credit != 3 && credit != 4 {
		return ErrInvalidCourseCredit
	}
	if status != 0 && status != 1 {
		return ErrInvalidCourseStatus
	}

	return nil
}

// toCourseDetail 把数据库模型转换成返回前端的 DTO。
func toCourseDetail(course model.Course) *CourseDetail {
	return &CourseDetail{
		ID:            course.ID,
		CourseName:    course.CourseName,
		TeacherName:   course.TeacherName,
		Capacity:      course.Capacity,
		SelectedCount: course.SelectedCount,
		TimeSlot:      course.TimeSlot,
		Credit:        course.Credit,
		Status:        course.Status,
		LikeCount:     course.LikeCount,
		CommentCount:  course.CommentCount,
		Version:       course.Version,
	}
}
