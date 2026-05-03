package handler

import (
	"errors"
	"net/http"
	"strings"

	"choose-course-backend/internal/pkg/errno"
	"choose-course-backend/internal/pkg/response"
	"choose-course-backend/internal/service"
	"github.com/gin-gonic/gin"
)

// SelectionHandler 负责 M5 抢课、退课、查询请求状态接口。
type SelectionHandler struct {
	selectionService      *service.SelectionService
	selectionAsyncService *service.SelectionAsyncService
}

// NewSelectionHandler 创建一个选课处理器。
func NewSelectionHandler(selectionService *service.SelectionService, selectionAsyncService *service.SelectionAsyncService) *SelectionHandler {
	return &SelectionHandler{
		selectionService:      selectionService,
		selectionAsyncService: selectionAsyncService,
	}
}

// SelectCourse 处理学生抢课请求。
//
// 这个 handler 自己不做业务判断，它主要负责：
// 1. 拿当前登录学生身份
// 2. 解析路径里的 courseId
// 3. 调用异步 service 先受理请求、写 pending 记录、投递 MQ
func (h *SelectionHandler) SelectCourse(c *gin.Context) {
	claims, ok := getStudentClaims(c)
	if !ok {
		return
	}

	courseID, ok := parseCourseID(c)
	if !ok {
		return
	}

	result, err := h.selectionAsyncService.SubmitSelectCourse(claims.UserID, courseID)
	if err != nil {
		handleSelectionError(c, err)
		return
	}

	response.Success(c, result)
}

// DropCourse 处理学生退课请求。
func (h *SelectionHandler) DropCourse(c *gin.Context) {
	claims, ok := getStudentClaims(c)
	if !ok {
		return
	}

	courseID, ok := parseCourseID(c)
	if !ok {
		return
	}

	result, err := h.selectionService.DropCourse(claims.UserID, courseID)
	if err != nil {
		handleSelectionError(c, err)
		return
	}

	response.Success(c, result)
}

// GetSelectionRequest 查询当前学生自己的选课请求记录。
func (h *SelectionHandler) GetSelectionRequest(c *gin.Context) {
	claims, ok := getStudentClaims(c)
	if !ok {
		return
	}

	requestNo := strings.TrimSpace(c.Param("requestNo"))
	if requestNo == "" {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	// M7 开始，这里优先走异步 service 的查询方法。
	// 它会在返回结果前，顺手处理“超时 pending 请求的自动收口”。
	result, err := h.selectionAsyncService.GetSelectionRequest(claims.UserID, requestNo)
	if err != nil {
		handleSelectionError(c, err)
		return
	}

	response.Success(c, result)
}

// handleSelectionError 统一把 service 层返回的错误转换成 HTTP 响应。
func handleSelectionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrStudentNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeStudentNotFound, errno.Message(errno.CodeStudentNotFound))
	case errors.Is(err, service.ErrUserDisabled):
		response.Fail(c, http.StatusForbidden, errno.CodeUserDisabled, errno.Message(errno.CodeUserDisabled))
	case errors.Is(err, service.ErrCourseNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeCourseNotFound, errno.Message(errno.CodeCourseNotFound))
	case errors.Is(err, service.ErrCourseClosed):
		response.Fail(c, http.StatusBadRequest, errno.CodeCourseClosed, errno.Message(errno.CodeCourseClosed))
	case errors.Is(err, service.ErrCourseFull):
		response.Fail(c, http.StatusBadRequest, errno.CodeCourseFull, errno.Message(errno.CodeCourseFull))
	case errors.Is(err, service.ErrCourseAlreadySelected):
		response.Fail(c, http.StatusConflict, errno.CodeCourseAlreadySelect, errno.Message(errno.CodeCourseAlreadySelect))
	case errors.Is(err, service.ErrSelectionTimeConflict):
		response.Fail(c, http.StatusBadRequest, errno.CodeCourseTimeConflict, errno.Message(errno.CodeCourseTimeConflict))
	case errors.Is(err, service.ErrInsufficientCredits):
		response.Fail(c, http.StatusBadRequest, errno.CodeCreditNotEnough, errno.Message(errno.CodeCreditNotEnough))
	case errors.Is(err, service.ErrCourseNotSelected):
		response.Fail(c, http.StatusBadRequest, errno.CodeCourseNotSelected, errno.Message(errno.CodeCourseNotSelected))
	case errors.Is(err, service.ErrSelectionRequestNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeSelectionReqNotFound, errno.Message(errno.CodeSelectionReqNotFound))
	default:
		response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
	}
}
