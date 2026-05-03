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

// AdminCourseHandler 负责管理员侧课程管理接口。
type AdminCourseHandler struct {
	courseService *service.AdminCourseService
}

// createCourseRequest 是新增课程请求体。
type createCourseRequest struct {
	CourseName  string `json:"course_name" binding:"required"`
	TeacherName string `json:"teacher_name" binding:"required"`
	Capacity    int    `json:"capacity" binding:"required"`
	TimeSlot    int8   `json:"time_slot" binding:"required"`
	Credit      int8   `json:"credit" binding:"required"`
	Status      int8   `json:"status" binding:"required"`
}

// updateCourseRequest 是修改课程请求体。
type updateCourseRequest struct {
	CourseName  *string `json:"course_name"`
	TeacherName *string `json:"teacher_name"`
	Capacity    *int    `json:"capacity"`
	TimeSlot    *int8   `json:"time_slot"`
	Credit      *int8   `json:"credit"`
	Status      *int8   `json:"status"`
}

// NewAdminCourseHandler 创建课程管理处理器。
func NewAdminCourseHandler(courseService *service.AdminCourseService) *AdminCourseHandler {
	return &AdminCourseHandler{courseService: courseService}
}

// CreateCourse 新增课程。
func (h *AdminCourseHandler) CreateCourse(c *gin.Context) {
	var req createCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	result, err := h.courseService.CreateCourse(service.CreateCourseInput{
		CourseName:  req.CourseName,
		TeacherName: req.TeacherName,
		Capacity:    req.Capacity,
		TimeSlot:    req.TimeSlot,
		Credit:      req.Credit,
		Status:      req.Status,
	})
	if err != nil {
		handleAdminCourseError(c, err)
		return
	}

	response.Success(c, result)
}

// ListCourses 查询课程列表。
func (h *AdminCourseHandler) ListCourses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	input := service.ListCoursesInput{
		Page:        page,
		PageSize:    pageSize,
		CourseName:  c.Query("course_name"),
		TeacherName: c.Query("teacher_name"),
	}

	// 下面这几项如果没传，就保持 nil，表示“不按这个条件过滤”。
	if statusStr := c.Query("status"); statusStr != "" {
		statusValue, err := strconv.Atoi(statusStr)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
			return
		}
		status := int8(statusValue)
		input.Status = &status
	}
	if slotStr := c.Query("time_slot"); slotStr != "" {
		slotValue, err := strconv.Atoi(slotStr)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
			return
		}
		timeSlot := int8(slotValue)
		input.TimeSlot = &timeSlot
	}
	if creditStr := c.Query("credit"); creditStr != "" {
		creditValue, err := strconv.Atoi(creditStr)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
			return
		}
		credit := int8(creditValue)
		input.Credit = &credit
	}

	result, err := h.courseService.ListCourses(input)
	if err != nil {
		handleAdminCourseError(c, err)
		return
	}

	response.Success(c, result)
}

// GetCourseByID 按课程 ID 查询详情。
func (h *AdminCourseHandler) GetCourseByID(c *gin.Context) {
	courseID, ok := parseCourseID(c)
	if !ok {
		return
	}

	result, err := h.courseService.GetCourseByID(courseID)
	if err != nil {
		handleAdminCourseError(c, err)
		return
	}

	response.Success(c, result)
}

// UpdateCourse 按课程 ID 修改课程。
func (h *AdminCourseHandler) UpdateCourse(c *gin.Context) {
	courseID, ok := parseCourseID(c)
	if !ok {
		return
	}

	var req updateCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	result, err := h.courseService.UpdateCourse(courseID, service.UpdateCourseInput{
		CourseName:  req.CourseName,
		TeacherName: req.TeacherName,
		Capacity:    req.Capacity,
		TimeSlot:    req.TimeSlot,
		Credit:      req.Credit,
		Status:      req.Status,
	})
	if err != nil {
		handleAdminCourseError(c, err)
		return
	}

	response.Success(c, result)
}

// parseCourseID 负责把路径里的 courseId 字符串安全转换成 uint64。
func parseCourseID(c *gin.Context) (uint64, bool) {
	courseID, err := strconv.ParseUint(c.Param("courseId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return 0, false
	}

	return courseID, true
}

// handleAdminCourseError 统一处理课程管理相关错误。
func handleAdminCourseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrCourseNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeCourseNotFound, errno.Message(errno.CodeCourseNotFound))
	case errors.Is(err, service.ErrInvalidCourseStatus):
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidCourseStatus, errno.Message(errno.CodeInvalidCourseStatus))
	case errors.Is(err, service.ErrInvalidCourseCredit):
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidCourseCredit, errno.Message(errno.CodeInvalidCourseCredit))
	case errors.Is(err, service.ErrInvalidCourseTimeSlot):
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidCourseSlot, errno.Message(errno.CodeInvalidCourseSlot))
	case errors.Is(err, service.ErrInvalidCourseCapacity):
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidCourseCap, errno.Message(errno.CodeInvalidCourseCap))
	case errors.Is(err, service.ErrCourseCannotClose):
		response.Fail(c, http.StatusBadRequest, errno.CodeCourseCannotClose, errno.Message(errno.CodeCourseCannotClose))
	case errors.Is(err, service.ErrCourseCapacityTooSmall):
		response.Fail(c, http.StatusBadRequest, errno.CodeCourseCapTooSmall, errno.Message(errno.CodeCourseCapTooSmall))
	case errors.Is(err, service.ErrCourseImmutableAfterSelected):
		response.Fail(c, http.StatusBadRequest, errno.CodeCourseLockedFields, errno.Message(errno.CodeCourseLockedFields))
	default:
		response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
	}
}
