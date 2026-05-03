package requestid

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gin-gonic/gin"
)

// 这两个常量统一定义了请求 ID 在 HTTP 头和 Gin 上下文中的名字。
const (
	HeaderName = "X-Request-ID"
	ContextKey = "request_id"
)

// Generate 生成一个请求 ID。
// 格式是“时间戳 + 随机串”，目的是既方便排查，又尽量避免重复。
func Generate() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}

	return time.Now().Format("20060102150405") + "-" + hex.EncodeToString(buf)
}

// FromContext 从 Gin 上下文中读取请求 ID。
func FromContext(c *gin.Context) string {
	if value, exists := c.Get(ContextKey); exists {
		if id, ok := value.(string); ok {
			return id
		}
	}

	return ""
}
