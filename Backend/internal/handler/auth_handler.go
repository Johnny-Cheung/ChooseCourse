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

// AuthHandler 负责认证接口：
// - 学生登录
// - 管理员登录
// - 获取当前登录用户信息
type AuthHandler struct {
	authService *service.AuthService
}

// studentLoginRequest 是学生登录请求体。
type studentLoginRequest struct {
	StudentNo string `json:"student_no" binding:"required"` // 学号
	Password  string `json:"password" binding:"required"`   // 明文密码
}

// adminLoginRequest 是管理员登录请求体。
type adminLoginRequest struct {
	AdminNo  string `json:"admin_no" binding:"required"` // 工号
	Password string `json:"password" binding:"required"` // 明文密码
}

// NewAuthHandler 创建认证处理器。
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// StudentLogin 处理学生登录接口。
func (h *AuthHandler) StudentLogin(c *gin.Context) {
	var req studentLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	result, err := h.authService.StudentLogin(req.StudentNo, req.Password)
	if err != nil {
		handleAuthError(c, err)
		return
	}

	response.Success(c, result)
}

// AdminLogin 处理管理员登录接口。
func (h *AuthHandler) AdminLogin(c *gin.Context) {
	var req adminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	result, err := h.authService.AdminLogin(req.AdminNo, req.Password)
	if err != nil {
		handleAuthError(c, err)
		return
	}

	response.Success(c, result)
}

// Me 返回当前登录用户信息。
// 这个接口依赖 JWT 鉴权中间件先把 Claims 放进 Gin 上下文。
func (h *AuthHandler) Me(c *gin.Context) {
	claims, ok := authjwt.FromContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, errno.Message(errno.CodeUnauthorized))
		return
	}

	result, err := h.authService.Me(claims)
	if err != nil {
		handleAuthError(c, err)
		return
	}

	response.Success(c, result)
}

// handleAuthError 统一把认证服务返回的错误转换成 HTTP 响应。
func handleAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		response.Fail(c, http.StatusUnauthorized, errno.CodeInvalidCredentials, errno.Message(errno.CodeInvalidCredentials))
	case errors.Is(err, service.ErrUserDisabled):
		response.Fail(c, http.StatusForbidden, errno.CodeUserDisabled, errno.Message(errno.CodeUserDisabled))
	case errors.Is(err, service.ErrUnsupportedRole):
		response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, errno.Message(errno.CodeUnauthorized))
	default:
		response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
	}
}
