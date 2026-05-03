package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"choose-course-backend/internal/pkg/errno"
	"choose-course-backend/internal/pkg/logger"
	"choose-course-backend/internal/pkg/response"
)

// Recovery 用来兜底捕获 panic。
// 如果某个处理函数发生 panic，程序不会直接崩溃，而是返回 500。
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.L().Error("panic recovered", logger.Any("error", recovered))
		response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
	})
}
