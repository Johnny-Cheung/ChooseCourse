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

// StudentCommentHandler 负责学生课程评论相关接口。
type StudentCommentHandler struct {
	commentService *service.StudentCommentService
}

// createCommentRequest 是发表评论请求体。
type createCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

// NewStudentCommentHandler 创建评论处理器。
func NewStudentCommentHandler(commentService *service.StudentCommentService) *StudentCommentHandler {
	return &StudentCommentHandler{commentService: commentService}
}

// ListCourseComments 查询某门课程的评论列表。
func (h *StudentCommentHandler) ListCourseComments(c *gin.Context) {
	claims, ok := getStudentClaims(c)
	if !ok {
		return
	}

	courseID, ok := parseCourseID(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	result, err := h.commentService.ListCourseComments(claims.UserID, courseID, service.CourseCommentListInput{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		handleStudentCommentError(c, err)
		return
	}

	response.Success(c, result)
}

// CreateComment 发表评论。
func (h *StudentCommentHandler) CreateComment(c *gin.Context) {
	claims, ok := getStudentClaims(c)
	if !ok {
		return
	}

	courseID, ok := parseCourseID(c)
	if !ok {
		return
	}

	var req createCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	result, err := h.commentService.CreateComment(claims.UserID, courseID, req.Content)
	if err != nil {
		handleStudentCommentError(c, err)
		return
	}

	response.Success(c, result)
}

// DeleteOwnComment 删除当前学生自己的评论。
func (h *StudentCommentHandler) DeleteOwnComment(c *gin.Context) {
	claims, ok := getStudentClaims(c)
	if !ok {
		return
	}

	commentID, ok := parseCommentID(c)
	if !ok {
		return
	}

	result, err := h.commentService.DeleteOwnComment(claims.UserID, commentID)
	if err != nil {
		handleStudentCommentError(c, err)
		return
	}

	response.Success(c, result)
}

// parseCommentID 把路径参数里的 commentId 转成 uint64。
func parseCommentID(c *gin.Context) (uint64, bool) {
	commentID, err := strconv.ParseUint(c.Param("commentId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return 0, false
	}

	return commentID, true
}

// handleStudentCommentError 统一处理评论相关错误。
func handleStudentCommentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrCourseNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeCourseNotFound, errno.Message(errno.CodeCourseNotFound))
	case errors.Is(err, service.ErrStudentNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeStudentNotFound, errno.Message(errno.CodeStudentNotFound))
	case errors.Is(err, service.ErrCommentNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeCommentNotFound, errno.Message(errno.CodeCommentNotFound))
	case errors.Is(err, service.ErrCommentForbidden):
		response.Fail(c, http.StatusForbidden, errno.CodeCommentForbidden, errno.Message(errno.CodeCommentForbidden))
	case errors.Is(err, service.ErrInvalidComment):
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidComment, errno.Message(errno.CodeInvalidComment))
	default:
		response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
	}
}
