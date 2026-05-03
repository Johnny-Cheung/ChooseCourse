package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"choose-course-backend/internal/mq"
	"choose-course-backend/internal/pkg/response"
	"choose-course-backend/internal/repository"
)

// Ping 是最简单的存活检查接口。
// 只要 HTTP 服务还活着，这个接口就会返回 pong。
func Ping(c *gin.Context) {
	response.Success(c, gin.H{"message": "pong"})
}

// Health 用于检查应用依赖是否正常。
// 当前会检查 MySQL、Redis 和 RabbitMQ 是否还能正常使用。
func Health(c *gin.Context) {
	mysqlErr := repository.PingMySQL()
	redisErr := repository.PingRedis(c.Request.Context())
	rabbitErr := mq.PingRabbitMQ()

	status := "ok"
	httpStatus := http.StatusOK

	// 只要有任意核心依赖不可用，就把整体状态标记为 degraded。
	if mysqlErr != nil || redisErr != nil || rabbitErr != nil {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	response.JSON(c, httpStatus, response.Payload{
		Code:    0,
		Message: status,
		Data: gin.H{
			"mysql":    mapHealth(mysqlErr),
			"redis":    mapHealth(redisErr),
			"rabbitmq": mapHealth(rabbitErr),
		},
	})
}

// mapHealth 把 error 转成简单的 up/down 状态，方便前端或监控读取。
func mapHealth(err error) string {
	if err != nil {
		return "down"
	}

	return "up"
}
