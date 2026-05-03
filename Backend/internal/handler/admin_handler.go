package handler

import (
	"net/http"

	"choose-course-backend/internal/pkg/errno"
	authjwt "choose-course-backend/internal/pkg/jwt"
	"choose-course-backend/internal/pkg/response"
	"choose-course-backend/internal/service"
	"github.com/gin-gonic/gin"
)

// AdminHandler 负责管理员“个人信息”相关接口。
type AdminHandler struct {
	adminService *service.AdminService
}

// updateAdminProfileRequest 表示管理员修改个人资料时的请求体。
type updateAdminProfileRequest struct {
	Name     *string `json:"name"`     // 姓名，可选
	Phone    *string `json:"phone"`    // 手机号，可选
	Password *string `json:"password"` // 新密码，可选
}

// NewAdminHandler 创建管理员个人信息处理器。
func NewAdminHandler(adminService *service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

// GetProfile 返回当前登录管理员的资料。
func (h *AdminHandler) GetProfile(c *gin.Context) {
	claims, ok := authjwt.FromContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, errno.Message(errno.CodeUnauthorized))
		return
	}

	result, err := h.adminService.GetProfile(claims.UserID)
	if err != nil {
		handleAdminProfileError(c, err)
		return
	}

	response.Success(c, result)
}

// UpdateProfile 修改当前登录管理员的资料。
func (h *AdminHandler) UpdateProfile(c *gin.Context) {
	claims, ok := authjwt.FromContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, errno.Message(errno.CodeUnauthorized))
		return
	}

	var req updateAdminProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	result, err := h.adminService.UpdateProfile(claims.UserID, service.UpdateAdminProfileInput{
		Name:     req.Name,
		Phone:    req.Phone,
		Password: req.Password,
	})
	if err != nil {
		handleAdminProfileError(c, err)
		return
	}

	response.Success(c, result)
}

// handleAdminProfileError 统一处理管理员个人资料相关错误。
func handleAdminProfileError(c *gin.Context, err error) {
	switch err {
	case service.ErrAdminNotFound:
		response.Fail(c, http.StatusNotFound, errno.CodeAdminNotFound, errno.Message(errno.CodeAdminNotFound))
	default:
		response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
	}
}
