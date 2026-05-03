package middleware

import (
	"net/http"

	"choose-course-backend/internal/pkg/errno"
	authjwt "choose-course-backend/internal/pkg/jwt"
	"choose-course-backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// RequireRole 用来限制某个接口只能被指定角色访问。
// 例如：
// - 学生接口只能 student 访问
// - 管理员接口只能 admin 访问
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := authjwt.FromContext(c)
		if !ok {
			response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, errno.Message(errno.CodeUnauthorized))
			c.Abort()
			return
		}

		if claims.Role != role {
			response.Fail(c, http.StatusForbidden, errno.CodeForbidden, errno.Message(errno.CodeForbidden))
			c.Abort()
			return
		}

		c.Next()
	}
}
