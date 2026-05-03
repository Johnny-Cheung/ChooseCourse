package handler

import (
	"errors"
	"net/http"
	"strconv"

	"choose-course-backend/internal/pkg/errno"
	"choose-course-backend/internal/pkg/response"
	"choose-course-backend/internal/service"
	"github.com/gin-gonic/gin"
)

// StudentCourseHandler 负责学生课程列表、搜索、详情和点赞。
type StudentCourseHandler struct {
	courseService *service.StudentCourseService
	likeService   *service.StudentLikeService
}

// NewStudentCourseHandler 创建学生课程处理器。
func NewStudentCourseHandler(courseService *service.StudentCourseService, likeService *service.StudentLikeService) *StudentCourseHandler {
	return &StudentCourseHandler{
		courseService: courseService,
		likeService:   likeService,
	}
}

// ListCourses 查询课程列表。
// 如果不传任何筛选条件，它会按分页返回全部课程。
func (h *StudentCourseHandler) ListCourses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	input := service.StudentCourseListInput{
		Page:        page,
		PageSize:    pageSize,
		CourseName:  c.Query("course_name"),
		TeacherName: c.Query("teacher_name"),
	}

	if statusStr := c.Query("status"); statusStr != "" {
		statusValue, err := strconv.Atoi(statusStr)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
			return
		}
		status := int8(statusValue)
		input.Status = &status
	}
	if slotStr := c.Query("time_slot"); slotStr != "" {
		slotValue, err := strconv.Atoi(slotStr)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
			return
		}
		timeSlot := int8(slotValue)
		input.TimeSlot = &timeSlot
	}
	if creditStr := c.Query("credit"); creditStr != "" {
		creditValue, err := strconv.Atoi(creditStr)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
			return
		}
		credit := int8(creditValue)
		input.Credit = &credit
	}

	result, err := h.courseService.ListCourses(input)
	if err != nil {
		handleStudentCourseError(c, err)
		return
	}

	response.Success(c, result)
}

// SearchCourses 搜索课程。
// 当前实现直接复用 ListCourses，因为搜索本质上也是列表查询的一种。
func (h *StudentCourseHandler) SearchCourses(c *gin.Context) {
	h.ListCourses(c)
}

// GetCourseByID 返回课程详情。
func (h *StudentCourseHandler) GetCourseByID(c *gin.Context) {
	courseID, ok := parseCourseID(c)
	if !ok {
		return
	}

	result, err := h.courseService.GetCourseByID(courseID)
	if err != nil {
		handleStudentCourseError(c, err)
		return
	}

	response.Success(c, result)
}

// LikeCourse 处理学生给课程点赞。
func (h *StudentCourseHandler) LikeCourse(c *gin.Context) {
	claims, ok := getStudentClaims(c)
	if !ok {
		return
	}

	courseID, ok := parseCourseID(c)
	if !ok {
		return
	}

	result, err := h.likeService.LikeCourse(claims.UserID, courseID)
	if err != nil {
		handleStudentCourseError(c, err)
		return
	}

	response.Success(c, result)
}

// UnlikeCourse 处理学生取消点赞。
func (h *StudentCourseHandler) UnlikeCourse(c *gin.Context) {
	claims, ok := getStudentClaims(c)
	if !ok {
		return
	}

	courseID, ok := parseCourseID(c)
	if !ok {
		return
	}

	result, err := h.likeService.UnlikeCourse(claims.UserID, courseID)
	if err != nil {
		handleStudentCourseError(c, err)
		return
	}

	response.Success(c, result)
}

// handleStudentCourseError 统一处理学生课程和点赞相关错误。
func handleStudentCourseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrCourseNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeCourseNotFound, errno.Message(errno.CodeCourseNotFound))
	case errors.Is(err, service.ErrLikeAlreadyExists):
		response.Fail(c, http.StatusConflict, errno.CodeLikeAlreadyExists, errno.Message(errno.CodeLikeAlreadyExists))
	case errors.Is(err, service.ErrLikeNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeLikeNotFound, errno.Message(errno.CodeLikeNotFound))
	default:
		response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
	}
}
