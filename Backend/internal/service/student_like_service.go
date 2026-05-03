package service

import (
	"context"
	"errors"
	"fmt"

	"choose-course-backend/internal/model"
	"choose-course-backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrLikeAlreadyExists 表示学生已经点赞过这门课。
	ErrLikeAlreadyExists = errors.New("like already exists")
	// ErrLikeNotFound 表示学生还没有给这门课点过赞。
	ErrLikeNotFound = errors.New("like not found")
)

// LikeActionResult 是点赞/取消点赞后的返回结果。
type LikeActionResult struct {
	CourseID  uint64 `json:"course_id"`
	LikeCount int    `json:"like_count"`
}

// StudentLikeService 负责课程点赞相关业务。
type StudentLikeService struct{}

// NewStudentLikeService 创建点赞服务。
func NewStudentLikeService() *StudentLikeService {
	return &StudentLikeService{}
}

// LikeCourse 处理点赞课程。
func (s *StudentLikeService) LikeCourse(studentID, courseID uint64) (*LikeActionResult, error) {
	var result *LikeActionResult
	var notificationSpec *StudentNotificationSpec

	err := repository.DB().Transaction(func(tx *gorm.DB) error {
		var course model.Course
		if err := tx.First(&course, courseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCourseNotFound
			}

			return err
		}

		like := model.CourseLike{
			StudentID: studentID,
			CourseID:  courseID,
		}

		if err := tx.Create(&like).Error; err != nil {
			if isDuplicateEntry(err) {
				return ErrLikeAlreadyExists
			}

			return err
		}

		if err := tx.Model(&course).
			UpdateColumn("like_count", gorm.Expr("like_count + ?", 1)).Error; err != nil {
			return err
		}
		if err := tx.Select("id", "course_name", "like_count").First(&course, courseID).Error; err != nil {
			return err
		}

		relatedCourseID := course.ID
		notificationSpec = newStudentNotificationSpec(
			newNotificationBizKey("like_success", fmt.Sprintf("%d", like.ID)),
			studentID,
			"like_success",
			"点赞成功",
			fmt.Sprintf("你已成功点赞课程《%s》。", course.CourseName),
			&relatedCourseID,
			nil,
		)

		result = &LikeActionResult{
			CourseID:  course.ID,
			LikeCount: course.LikeCount,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	publishStudentNotificationBestEffort(context.Background(), notificationSpec)

	return result, nil
}

// UnlikeCourse 处理取消点赞。
func (s *StudentLikeService) UnlikeCourse(studentID, courseID uint64) (*LikeActionResult, error) {
	var result *LikeActionResult
	var notificationSpec *StudentNotificationSpec

	err := repository.DB().Transaction(func(tx *gorm.DB) error {
		var course model.Course
		if err := tx.First(&course, courseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCourseNotFound
			}

			return err
		}

		var like model.CourseLike
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("student_id = ? AND course_id = ?", studentID, courseID).
			First(&like).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrLikeNotFound
			}

			return err
		}

		if err := tx.Delete(&like).Error; err != nil {
			return err
		}

		if err := tx.Model(&course).
			UpdateColumn("like_count", gorm.Expr("CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END")).Error; err != nil {
			return err
		}
		if err := tx.Select("id", "course_name", "like_count").First(&course, courseID).Error; err != nil {
			return err
		}

		relatedCourseID := course.ID
		notificationSpec = newStudentNotificationSpec(
			newNotificationBizKey("unlike_success", fmt.Sprintf("%d", like.ID)),
			studentID,
			"unlike_success",
			"取消点赞成功",
			fmt.Sprintf("你已取消点赞课程《%s》。", course.CourseName),
			&relatedCourseID,
			nil,
		)

		result = &LikeActionResult{
			CourseID:  course.ID,
			LikeCount: course.LikeCount,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	publishStudentNotificationBestEffort(context.Background(), notificationSpec)

	return result, nil
}
