package handler

import (
	"errors"
	"net/http"
	"strconv"

	"choose-course-backend/internal/pkg/errno"
	"choose-course-backend/internal/pkg/response"
	"choose-course-backend/internal/service"
	"github.com/gin-gonic/gin"
)

// StudentNotificationHandler 负责学生消息中心接口。
type StudentNotificationHandler struct {
	notificationService *service.StudentNotificationService
}

// NewStudentNotificationHandler 创建消息处理器。
func NewStudentNotificationHandler(notificationService *service.StudentNotificationService) *StudentNotificationHandler {
	return &StudentNotificationHandler{notificationService: notificationService}
}

// ListNotifications 查询当前学生自己的通知列表。
//
// 这个 handler 自己不直接查数据库，它主要负责 3 件事：
// 1. 从 Gin 上下文里拿到当前登录学生的身份信息
// 2. 读取并整理查询参数，比如 page、page_size、is_read
// 3. 调用 service 层查询通知列表，再把结果返回给前端
func (h *StudentNotificationHandler) ListNotifications(c *gin.Context) {
	// 先拿当前登录学生的 claims。
	// 这里的 claims 是前面 JWT 中间件解析 token 后放进 Gin 上下文里的。
	// 如果拿不到，说明当前请求没有通过登录鉴权，getStudentClaims 内部会直接返回 401。
	claims, ok := getStudentClaims(c)
	if !ok {
		return
	}

	// 从查询参数里读取分页信息。
	// c.DefaultQuery 的意思是：
	// - 如果前端传了 page/page_size，就用前端传的值
	// - 如果没传，就使用默认值 "1" 和 "10"
	//
	// 这里用 strconv.Atoi 把字符串转成 int。
	// 如果转换失败，Atoi 会返回错误；但这里暂时忽略错误，
	// 因为 service 层还会再对 page/pageSize 做一次兜底处理。
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	// 先把最基础的分页参数组装成 service 层需要的输入结构。
	input := service.NotificationListInput{
		Page:     page,
		PageSize: pageSize,
	}

	// is_read 是一个可选过滤条件：
	// - 不传：查询全部通知
	// - 传 0：只查未读通知
	// - 传 1：只查已读通知
	//
	// 之所以写成指针字段，是为了区分：
	// - nil：调用方根本没传这个条件
	// - 0/1：调用方明确传了过滤值
	if isReadStr := c.Query("is_read"); isReadStr != "" {
		isReadValue, err := strconv.Atoi(isReadStr)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
			return
		}

		// service 层结构体里定义的是 *int8，
		// 所以这里把转换后的 int 再转成 int8，并取地址传进去。
		isRead := int8(isReadValue)
		input.IsRead = &isRead
	}

	// 调用 service 层真正执行查询。
	// 注意这里传入的是 claims.UserID，而不是前端传来的学生 ID。
	// 这样可以保证学生只能查自己的通知，不能越权查别人的通知。
	result, err := h.notificationService.ListNotifications(claims.UserID, input)
	if err != nil {
		handleStudentNotificationError(c, err)
		return
	}

	// 查询成功后，统一按项目里的标准成功响应格式返回。
	response.Success(c, result)
}

// MarkRead 把当前学生的一条通知标记为已读。
func (h *StudentNotificationHandler) MarkRead(c *gin.Context) {
	claims, ok := getStudentClaims(c)
	if !ok {
		return
	}

	notificationID, ok := parseNotificationID(c)
	if !ok {
		return
	}

	result, err := h.notificationService.MarkRead(claims.UserID, notificationID)
	if err != nil {
		handleStudentNotificationError(c, err)
		return
	}

	response.Success(c, result)
}

// parseNotificationID 把路径参数里的 notificationId 转成 uint64。
func parseNotificationID(c *gin.Context) (uint64, bool) {
	notificationID, err := strconv.ParseUint(c.Param("notificationId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return 0, false
	}

	return notificationID, true
}

// handleStudentNotificationError 统一处理消息中心相关错误。
func handleStudentNotificationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotificationNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeNotificationNotFound, errno.Message(errno.CodeNotificationNotFound))
	default:
		response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
	}
}
