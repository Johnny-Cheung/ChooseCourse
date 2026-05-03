package handler

import (
	"net/http"

	"choose-course-backend/internal/pkg/errno"
	"choose-course-backend/internal/pkg/response"
	"choose-course-backend/internal/service"
	"github.com/gin-gonic/gin"
)

// TimetableHandler 负责“查看当前学生课表”接口。
type TimetableHandler struct {
	timetableService *service.TimetableService
}

// NewTimetableHandler 创建课表处理器。
func NewTimetableHandler(timetableService *service.TimetableService) *TimetableHandler {
	return &TimetableHandler{timetableService: timetableService}
}

// GetTimetable 查询当前登录学生的课表。
//
// 这里的思路和其他学生端接口一致：
// 1. 先从上下文里拿当前学生身份
// 2. 再调用 service 层查数据库
// 3. 最后统一返回成功响应
func (h *TimetableHandler) GetTimetable(c *gin.Context) {
	claims, ok := getStudentClaims(c)
	if !ok {
		return
	}

	result, err := h.timetableService.GetTimetable(claims.UserID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
		return
	}

	response.Success(c, result)
}
