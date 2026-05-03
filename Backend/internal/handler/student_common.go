package handler

import (
	"net/http"

	"choose-course-backend/internal/pkg/errno"
	authjwt "choose-course-backend/internal/pkg/jwt"
	"choose-course-backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// getStudentClaims 是学生端 handler 常用的小工具函数。
// 作用是：
// 1. 从 Gin 上下文拿 JWT claims
// 2. 如果没拿到，就直接返回 401
func getStudentClaims(c *gin.Context) (*authjwt.Claims, bool) {
	claims, ok := authjwt.FromContext(c)
	if !ok {
		response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, errno.Message(errno.CodeUnauthorized))
		return nil, false
	}

	return claims, true
}
