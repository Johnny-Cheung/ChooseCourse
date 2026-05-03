package handler

import (
	"errors"
	"net/http"

	"choose-course-backend/internal/pkg/errno"
	authjwt "choose-course-backend/internal/pkg/jwt"
	"choose-course-backend/internal/pkg/response"
	"choose-course-backend/internal/service"
	"github.com/gin-gonic/gin"
)

// StudentProfileHandler 负责学生“查看和修改个人资料”接口。
type StudentProfileHandler struct {
	profileService *service.StudentProfileService
}

// updateStudentProfileRequest 是学生修改个人资料时的请求体。
// 这里不提供姓名和学号字段，因为题目要求它们不可修改。
type updateStudentProfileRequest struct {
	Phone    *string `json:"phone"`
	Password *string `json:"password"`
}

// NewStudentProfileHandler 创建学生资料处理器。
func NewStudentProfileHandler(profileService *service.StudentProfileService) *StudentProfileHandler {
	return &StudentProfileHandler{profileService: profileService}
}

// GetProfile 返回当前登录学生自己的资料。
func (h *StudentProfileHandler) GetProfile(c *gin.Context) {
	claims, ok := authjwt.FromContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, errno.Message(errno.CodeUnauthorized))
		return
	}

	result, err := h.profileService.GetProfile(claims.UserID)
	if err != nil {
		handleStudentProfileError(c, err)
		return
	}

	response.Success(c, result)
}

// UpdateProfile 修改当前登录学生的资料。
func (h *StudentProfileHandler) UpdateProfile(c *gin.Context) {
	claims, ok := authjwt.FromContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, errno.Message(errno.CodeUnauthorized))
		return
	}

	var req updateStudentProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	result, err := h.profileService.UpdateProfile(claims.UserID, service.UpdateStudentProfileInput{
		Phone:    req.Phone,
		Password: req.Password,
	})
	if err != nil {
		handleStudentProfileError(c, err)
		return
	}

	response.Success(c, result)
}

// handleStudentProfileError 统一处理学生资料相关错误。
func handleStudentProfileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrStudentNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeStudentNotFound, errno.Message(errno.CodeStudentNotFound))
	default:
		response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
	}
}
