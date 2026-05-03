package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"choose-course-backend/internal/pkg/errno"
	"choose-course-backend/internal/pkg/requestid"
)

// Payload 是项目统一的 HTTP 响应结构。
// 后续所有接口都建议返回这个结构，保证前后端交互一致。
type Payload struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// Success 返回一个标准成功响应。
func Success(c *gin.Context, data any) {
	JSON(c, http.StatusOK, Payload{
		Code:    errno.CodeSuccess,
		Message: errno.Message(errno.CodeSuccess),
		Data:    data,
	})
}

// Fail 返回一个标准失败响应。
func Fail(c *gin.Context, httpStatus int, code int, message string) {
	JSON(c, httpStatus, Payload{
		Code:    code,
		Message: message,
	})
}

// JSON 是最底层的统一输出函数。
// 这里会自动把 request_id 填到响应里。
func JSON(c *gin.Context, httpStatus int, payload Payload) {
	payload.RequestID = requestid.FromContext(c)
	c.JSON(httpStatus, payload)
}
