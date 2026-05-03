package middleware

import "github.com/gin-gonic/gin"

// CORS 处理跨域响应头。
// 前后端分离开发时，浏览器会依赖这些响应头决定是否允许跨域访问。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("Access-Control-Allow-Origin", "*")
		header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		header.Set("Access-Control-Expose-Headers", "X-Request-ID")

		// 浏览器的预检请求只需要返回 204，不必继续走后面的业务处理。
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
