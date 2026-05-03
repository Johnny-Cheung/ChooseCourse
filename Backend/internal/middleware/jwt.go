package middleware

import (
	"net/http"
	"strings"

	"choose-course-backend/internal/pkg/errno"
	authjwt "choose-course-backend/internal/pkg/jwt"
	"choose-course-backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// JWTAuth 负责校验 Authorization 请求头里的 Bearer Token。
// 它只负责“身份令牌是否有效”这件事：
// 1. 请求头格式是否正确
// 2. token 是否能成功解析
// 3. 解析后的 claims 放进 Gin 上下文
//
// 至于“账号当前是否被禁用”，交给单独的 RequireActiveUser 中间件处理。
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, errno.Message(errno.CodeUnauthorized))
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, "invalid authorization header")
			c.Abort()
			return
		}

		claims, err := authjwt.ParseToken(parts[1])
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set(authjwt.ContextKeyClaims, claims)
		c.Next()
	}
}
