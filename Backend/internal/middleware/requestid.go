package middleware

import (
	"github.com/gin-gonic/gin"

	"choose-course-backend/internal/pkg/requestid"
)

// RequestID 给每个请求分配唯一编号。
// 这个编号会被放进上下文和响应头里，方便日志追踪。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果上游已经传了请求 ID，就沿用上游的。
		id := c.GetHeader(requestid.HeaderName)
		if id == "" {
			id = requestid.Generate()
		}

		// 既写入 Gin 上下文，也写入响应头，供业务代码和客户端使用。
		c.Set(requestid.ContextKey, id)
		c.Writer.Header().Set(requestid.HeaderName, id)
		c.Next()
	}
}
