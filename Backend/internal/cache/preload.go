package cache

import (
	"context"
	"strconv"

	"choose-course-backend/internal/model"
	"choose-course-backend/internal/repository"
	"github.com/redis/go-redis/v9"
)

const selectionPreloadBatchSize = 500

// SelectionPreloadResult 记录一次选课缓存预热的统计结果。
type SelectionPreloadResult struct {
	OpenCoursesLoaded        int `json:"open_courses_loaded"`
	ClosedCoursesInvalidated int `json:"closed_courses_invalidated"`
	StudentsLoaded           int `json:"students_loaded"`
}

// PreloadSelectionCaches 批量预热选课相关缓存。
//
// 当前策略是：
// 1. 预热所有开课课程的选课缓存
// 2. 删除不开课课程的旧选课缓存，避免 Redis 残留脏状态
// 3. 预热所有学生的选课缓存
func PreloadSelectionCaches(ctx context.Context) (*SelectionPreloadResult, error) {
	client, err := redisClient()
	if err != nil {
		return nil, err
	}

	result := &SelectionPreloadResult{}

	if err := preloadOpenCourseSelectionCaches(ctx, client, result); err != nil {
		return nil, err
	}
	if err := invalidateClosedCourseSelectionCaches(ctx, client, result); err != nil {
		return nil, err
	}
	if err := preloadStudentSelectionCaches(ctx, client, result); err != nil {
		return nil, err
	}

	return result, nil
}

// preloadOpenCourseSelectionCaches 预热所有“开课状态”的课程缓存。
func preloadOpenCourseSelectionCaches(ctx context.Context, client *redis.Client, result *SelectionPreloadResult) error {
	var lastID uint64

	for {
		var courses []model.Course
		if err := repository.DB().
			Select("id", "capacity", "selected_count", "status", "credit", "time_slot").
			Where("status = ? AND id > ?", 1, lastID).
			Order("id ASC").
			Limit(selectionPreloadBatchSize).
			Find(&courses).Error; err != nil {
			return err
		}

		if len(courses) == 0 {
			return nil
		}

		pipe := client.Pipeline()
		for _, course := range courses {
			remainingStock := course.Capacity - course.SelectedCount
			if remainingStock < 0 {
				remainingStock = 0
			}

			pipe.Set(ctx, CourseStockKey(course.ID), remainingStock, 0)
			pipe.Set(ctx, CourseStatusKey(course.ID), course.Status, 0)
			pipe.Set(ctx, CourseCreditKey(course.ID), course.Credit, 0)
			pipe.Set(ctx, CourseSlotKey(course.ID), course.TimeSlot, 0)
			pipe.Del(ctx, CourseMissingKey(course.ID))
		}

		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}

		result.OpenCoursesLoaded += len(courses)
		lastID = courses[len(courses)-1].ID
	}
}

// invalidateClosedCourseSelectionCaches 删除所有“不开课课程”的旧选课缓存。
func invalidateClosedCourseSelectionCaches(ctx context.Context, client *redis.Client, result *SelectionPreloadResult) error {
	var lastID uint64

	for {
		var courses []struct {
			ID uint64 `gorm:"column:id"`
		}
		if err := repository.DB().
			Model(&model.Course{}).
			Select("id").
			Where("status <> ? AND id > ?", 1, lastID).
			Order("id ASC").
			Limit(selectionPreloadBatchSize).
			Scan(&courses).Error; err != nil {
			return err
		}

		if len(courses) == 0 {
			return nil
		}

		pipe := client.Pipeline()
		for _, course := range courses {
			pipe.Del(
				ctx,
				CourseStockKey(course.ID),
				CourseStatusKey(course.ID),
				CourseCreditKey(course.ID),
				CourseSlotKey(course.ID),
				CourseMissingKey(course.ID),
			)
		}

		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}

		result.ClosedCoursesInvalidated += len(courses)
		lastID = courses[len(courses)-1].ID
	}
}

// preloadStudentSelectionCaches 预热所有学生的选课缓存。
func preloadStudentSelectionCaches(ctx context.Context, client *redis.Client, result *SelectionPreloadResult) error {
	var lastID uint64

	for {
		var students []model.Student
		if err := repository.DB().
			Select("id", "credit_used", "credit_limit").
			Where("id > ?", lastID).
			Order("id ASC").
			Limit(selectionPreloadBatchSize).
			Find(&students).Error; err != nil {
			return err
		}

		if len(students) == 0 {
			return nil
		}

		studentIDs := make([]uint64, 0, len(students))
		for _, student := range students {
			studentIDs = append(studentIDs, student.ID)
		}

		var selectedRows []struct {
			StudentID uint64 `gorm:"column:student_id"`
			CourseID  uint64 `gorm:"column:course_id"`
			TimeSlot  int8   `gorm:"column:time_slot"`
		}
		if err := repository.DB().
			Table("enrollments").
			Select("enrollments.student_id, enrollments.course_id, courses.time_slot").
			Joins("JOIN courses ON courses.id = enrollments.course_id").
			Where("enrollments.student_id IN ? AND enrollments.status = 1", studentIDs).
			Scan(&selectedRows).Error; err != nil {
			return err
		}

		selectedMembers := make(map[uint64][]any, len(students))
		slotBitmaps := make(map[uint64]uint64, len(students))

		for _, student := range students {
			selectedMembers[student.ID] = []any{selectedSetSentinel}
		}

		for _, row := range selectedRows {
			selectedMembers[row.StudentID] = append(selectedMembers[row.StudentID], strconv.FormatUint(row.CourseID, 10))
			slotBitmaps[row.StudentID] |= timeSlotBitmap(row.TimeSlot)
		}

		pipe := client.Pipeline()
		for _, student := range students {
			selectedKey := StudentSelectedKey(student.ID)

			pipe.Del(ctx, selectedKey)
			pipe.SAdd(ctx, selectedKey, selectedMembers[student.ID]...)
			pipe.Set(ctx, StudentCreditUsedKey(student.ID), student.CreditUsed, 0)
			pipe.Set(ctx, StudentCreditLimitKey(student.ID), student.CreditLimit, 0)
			pipe.Set(ctx, StudentSlotBitmapKey(student.ID), slotBitmaps[student.ID], 0)
			pipe.Del(ctx, StudentMissingKey(student.ID))
		}

		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}

		result.StudentsLoaded += len(students)
		lastID = students[len(students)-1].ID
	}
}
