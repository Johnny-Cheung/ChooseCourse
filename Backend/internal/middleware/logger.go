package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"choose-course-backend/internal/pkg/logger"
	"choose-course-backend/internal/pkg/requestid"
)

// Logger 记录每一次 HTTP 请求的基础信息。
// 这样后面排查接口报错、耗时过高时会方便很多。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求进入中间件时的时间，用来统计耗时。
		start := time.Now()

		// 继续执行后续中间件和真正的业务处理函数。
		c.Next()

		// 请求结束后再统一打印本次请求的日志。
		logger.L().Info("http request",
			logger.String("request_id", requestid.FromContext(c)),
			logger.String("method", c.Request.Method),
			logger.String("path", c.FullPath()),
			logger.Int("status", c.Writer.Status()),
			logger.String("client_ip", c.ClientIP()),
			logger.Duration("latency", time.Since(start)),
		)
	}
}
