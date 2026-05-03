package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"choose-course-backend/internal/model"
	"choose-course-backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrCommentNotFound 表示评论不存在或已被删除。
	ErrCommentNotFound = errors.New("comment not found")
	// ErrCommentForbidden 表示当前学生不能删除这条评论。
	ErrCommentForbidden = errors.New("comment forbidden")
	// ErrInvalidComment 表示评论内容为空或过长。
	ErrInvalidComment = errors.New("invalid comment")
)

// CourseCommentItem 是评论列表和评论详情统一返回结构。
type CourseCommentItem struct {
	ID          uint64    `json:"id"`
	CourseID    uint64    `json:"course_id"`
	StudentID   uint64    `json:"student_id"`
	StudentName string    `json:"student_name"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
	IsMine      bool      `json:"is_mine"`
}

// CourseCommentListInput 是评论列表分页参数。
type CourseCommentListInput struct {
	Page     int
	PageSize int
}

// CourseCommentListResult 是评论列表返回结构。
type CourseCommentListResult struct {
	List     []CourseCommentItem `json:"list"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// DeleteCommentResult 是删除评论后的返回结构。
type DeleteCommentResult struct {
	CommentID    uint64 `json:"comment_id"`
	CourseID     uint64 `json:"course_id"`
	CommentCount int    `json:"comment_count"`
}

// StudentCommentService 负责课程评论相关业务。
type StudentCommentService struct{}

// NewStudentCommentService 创建评论服务。
func NewStudentCommentService() *StudentCommentService {
	return &StudentCommentService{}
}

// ListCourseComments 查询某门课的评论列表。
//
// 这个方法做的事情可以拆成 4 步：
// 1. 先确认课程存在，避免对一个不存在的课程返回“空评论列表”
// 2. 规范化分页参数，保证 page/pageSize 不会出现非法值
// 3. 先统计评论总数，再查询当前页的评论数据
// 4. 把数据库结果转换成前端使用的返回结构，并标记哪条评论是“我自己发的”
func (s *StudentCommentService) ListCourseComments(studentID, courseID uint64, input CourseCommentListInput) (*CourseCommentListResult, error) {
	// 先确认课程存在。
	// 这样如果 courseID 传错了，前端拿到的是“课程不存在”，
	// 而不是一个语义不清的空列表。
	var course model.Course
	if err := repository.DB().First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCourseNotFound
		}

		return nil, err
	}

	// 对分页参数做兜底：
	// - page 默认从 1 开始
	// - pageSize 默认 10
	// - pageSize 最大 100，避免一次查太多数据
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

	// 先把“这门课下的有效评论”这个基础查询条件提出来，
	// 后面统计总数时可以直接复用。
	// status = 1 表示评论仍然是正常可见状态。
	db := repository.DB().Model(&model.CourseComment{}).Where("course_id = ? AND status = 1", courseID)

	// total 是总评论数，不是当前页条数。
	// 前端做分页时通常需要它来计算总页数。
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	// 这里没有直接用 model.CourseComment 来接结果，
	// 是因为列表里还需要“评论人姓名” student_name，
	// 这个字段来自 students 表，所以这里定义一个临时结构体来接联表查询结果。
	var comments []struct {
		ID          uint64    `gorm:"column:id"`
		CourseID    uint64    `gorm:"column:course_id"`
		StudentID   uint64    `gorm:"column:student_id"`
		StudentName string    `gorm:"column:student_name"`
		Content     string    `gorm:"column:content"`
		CreatedAt   time.Time `gorm:"column:created_at"`
	}

	// 查询当前页的评论数据。
	// 这里通过 JOIN students 把评论人的姓名一并查出来，
	// 同时按创建时间倒序排列，让最新评论排在前面。
	if err := repository.DB().
		Table("course_comments").
		Select("course_comments.id, course_comments.course_id, course_comments.student_id, students.name AS student_name, course_comments.content, course_comments.created_at").
		Joins("JOIN students ON students.id = course_comments.student_id").
		Where("course_comments.course_id = ? AND course_comments.status = 1", courseID).
		Order("course_comments.created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&comments).Error; err != nil {
		return nil, err
	}

	// 把数据库查询结果转换成接口返回结构。
	// 注意 IsMine 这个字段：
	// 它不是数据库直接存的，而是根据“评论的 student_id 是否等于当前登录学生的 studentID”算出来的。
	// 这样前端就能知道哪些评论是自己发的，从而决定是否显示“删除”按钮。
	list := make([]CourseCommentItem, 0, len(comments))
	for _, comment := range comments {
		list = append(list, CourseCommentItem{
			ID:          comment.ID,
			CourseID:    comment.CourseID,
			StudentID:   comment.StudentID,
			StudentName: comment.StudentName,
			Content:     comment.Content,
			CreatedAt:   comment.CreatedAt,
			IsMine:      comment.StudentID == studentID,
		})
	}

	// 最后把“当前页数据 + 总数 + 分页参数”一起返回给前端。
	return &CourseCommentListResult{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// CreateComment 发表评论。
//
// 这个方法做的事情可以拆成 5 步：
// 1. 先处理并校验评论内容，防止空评论或超长评论
// 2. 开启数据库事务，保证“写评论 + 更新课程评论数 + 写通知”要么一起成功，要么一起失败
// 3. 确认课程存在、学生存在
// 4. 插入评论记录，并把课程表里的 comment_count 加 1
// 5. 给当前学生写一条“评论成功”的通知，同时返回新评论数据
func (s *StudentCommentService) CreateComment(studentID, courseID uint64, content string) (*CourseCommentItem, error) {
	// strings.TrimSpace 会把前后空格、换行等空白字符去掉。
	// 这样像 "   " 这种内容就不会被当成有效评论。
	trimmedContent := strings.TrimSpace(content)

	// 评论内容为空，或者长度超过 500 个字符，都直接返回业务错误。
	// 这里用 utf8.RuneCountInString，是为了按“字符数”而不是“字节数”计数，
	// 这样中文不会因为占多个字节而被错误计算。
	if trimmedContent == "" || utf8.RuneCountInString(trimmedContent) > 500 {
		return nil, ErrInvalidComment
	}

	// result 用来接收事务成功后的最终返回结果。
	// 之所以先声明在外面，是因为事务函数内部生成了评论 ID 和时间，
	// 这些数据要带到事务外面作为最终返回值。
	var result *CourseCommentItem
	var notificationSpec *StudentNotificationSpec

	// 为什么这里要开事务：
	// 发表评论不是只做一件事，而是同时影响多张表/多个字段：
	// 1. course_comments 新增一条评论
	// 2. courses.comment_count 加 1
	// 3. notifications 新增一条通知
	// 如果中途有一步失败，就应该整体回滚，避免数据不一致。
	err := repository.DB().Transaction(func(tx *gorm.DB) error {
		// 先确认课程存在。
		// 如果课程都不存在，就不能往它下面发表评论。
		var course model.Course
		if err := tx.First(&course, courseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCourseNotFound
			}

			return err
		}

		// 再确认当前学生存在。
		// 正常情况下 studentID 来自已登录用户，但这里再查一次会更稳妥。
		var student model.Student
		if err := tx.First(&student, studentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStudentNotFound
			}

			return err
		}

		// 组装准备插入数据库的评论对象。
		// Status = 1 表示这是一条正常可见的评论。
		comment := model.CourseComment{
			StudentID: studentID,
			CourseID:  courseID,
			Content:   trimmedContent,
			Status:    1,
		}

		if err := tx.Create(&comment).Error; err != nil {
			return err
		}

		// 评论写入成功后，要同步维护课程表里的 comment_count 冗余计数字段。
		// 这样后面课程列表和课程详情就能直接显示评论数，不用每次现查评论表再 count。
		if err := tx.Model(&course).
			UpdateColumn("comment_count", gorm.Expr("comment_count + ?", 1)).Error; err != nil {
			return err
		}
		if err := tx.Select("id", "course_name", "comment_count").First(&course, courseID).Error; err != nil {
			return err
		}

		relatedCourseID := course.ID
		relatedCommentID := comment.ID
		notificationSpec = newStudentNotificationSpec(
			newNotificationBizKey("comment_create_success", fmt.Sprintf("%d", comment.ID)),
			studentID,
			"comment_create_success",
			"评论成功",
			fmt.Sprintf("你已成功评论课程《%s》。", course.CourseName),
			&relatedCourseID,
			&relatedCommentID,
		)

		// 把数据库里的新评论记录转换成接口返回结构。
		// 这里 IsMine 直接写 true，因为这条评论就是当前学生刚刚创建的。
		result = &CourseCommentItem{
			ID:          comment.ID,
			CourseID:    comment.CourseID,
			StudentID:   comment.StudentID,
			StudentName: student.Name,
			Content:     comment.Content,
			CreatedAt:   comment.CreatedAt,
			IsMine:      true,
		}

		return nil
	})

	// 只要事务里的任一步失败，这里就会拿到错误。
	if err != nil {
		return nil, err
	}

	publishStudentNotificationBestEffort(context.Background(), notificationSpec)

	// 事务全部成功后，再把最终结果返回给调用方。
	return result, nil
}

// DeleteOwnComment 删除自己的评论。
func (s *StudentCommentService) DeleteOwnComment(studentID, commentID uint64) (*DeleteCommentResult, error) {
	var result *DeleteCommentResult
	var notificationSpec *StudentNotificationSpec

	err := repository.DB().Transaction(func(tx *gorm.DB) error {
		var comment model.CourseComment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&comment, commentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCommentNotFound
			}

			return err
		}

		if comment.Status != 1 {
			return ErrCommentNotFound
		}
		if comment.StudentID != studentID {
			return ErrCommentForbidden
		}

		var course model.Course
		if err := tx.First(&course, comment.CourseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCourseNotFound
			}

			return err
		}

		if err := tx.Model(&comment).Updates(map[string]any{"status": 0}).Error; err != nil {
			return err
		}

		if err := tx.Model(&course).
			UpdateColumn("comment_count", gorm.Expr("CASE WHEN comment_count > 0 THEN comment_count - 1 ELSE 0 END")).Error; err != nil {
			return err
		}
		if err := tx.Select("id", "course_name", "comment_count").First(&course, comment.CourseID).Error; err != nil {
			return err
		}

		relatedCourseID := course.ID
		relatedCommentID := comment.ID
		notificationSpec = newStudentNotificationSpec(
			newNotificationBizKey("comment_delete_success", fmt.Sprintf("%d", comment.ID)),
			studentID,
			"comment_delete_success",
			"删除评论成功",
			fmt.Sprintf("你已删除课程《%s》下的一条评论。", course.CourseName),
			&relatedCourseID,
			&relatedCommentID,
		)

		result = &DeleteCommentResult{
			CommentID:    comment.ID,
			CourseID:     comment.CourseID,
			CommentCount: course.CommentCount,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	publishStudentNotificationBestEffort(context.Background(), notificationSpec)

	return result, nil
}
