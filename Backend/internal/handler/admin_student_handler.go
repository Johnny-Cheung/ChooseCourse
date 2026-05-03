package handler

import (
	"errors"
	"net/http"

	"choose-course-backend/internal/pkg/errno"
	"choose-course-backend/internal/pkg/response"
	"choose-course-backend/internal/service"
	"github.com/gin-gonic/gin"
)

// AdminStudentHandler 负责管理员侧的学生管理接口。
type AdminStudentHandler struct {
	studentService *service.AdminStudentService
}

// createStudentRequest 是新增学生请求体。
type createStudentRequest struct {
	StudentNo   string `json:"student_no" binding:"required"` // 学号
	Password    string `json:"password" binding:"required"`   // 初始密码
	Name        string `json:"name" binding:"required"`       // 姓名
	Phone       string `json:"phone" binding:"required"`      // 手机号
	CreditLimit int    `json:"credit_limit"`                  // 学分上限，可选，默认 25
	Status      int8   `json:"status"`                        // 状态，可选，默认 1
}

// updateStudentRequest 是修改学生请求体。
// 所有字段都可选，只更新传进来的字段。
type updateStudentRequest struct {
	Name        *string `json:"name"`
	Phone       *string `json:"phone"`
	Password    *string `json:"password"`
	CreditLimit *int    `json:"credit_limit"`
	Status      *int8   `json:"status"`
}

// NewAdminStudentHandler 创建学生管理处理器。
func NewAdminStudentHandler(studentService *service.AdminStudentService) *AdminStudentHandler {
	return &AdminStudentHandler{studentService: studentService}
}

// CreateStudent 新增学生。
func (h *AdminStudentHandler) CreateStudent(c *gin.Context) {
	var req createStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	result, err := h.studentService.CreateStudent(service.CreateStudentInput{
		StudentNo:   req.StudentNo,
		Password:    req.Password,
		Name:        req.Name,
		Phone:       req.Phone,
		CreditLimit: req.CreditLimit,
		Status:      req.Status,
	})
	if err != nil {
		handleAdminStudentError(c, err)
		return
	}

	response.Success(c, result)
}

// GetStudentByNo 按学号查询学生信息。
func (h *AdminStudentHandler) GetStudentByNo(c *gin.Context) {
	studentNo := c.Param("studentNo")
	if studentNo == "" {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	result, err := h.studentService.GetStudentByNo(studentNo)
	if err != nil {
		handleAdminStudentError(c, err)
		return
	}

	response.Success(c, result)
}

// UpdateStudentByNo 按学号修改学生信息。
func (h *AdminStudentHandler) UpdateStudentByNo(c *gin.Context) {
	studentNo := c.Param("studentNo")
	if studentNo == "" {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	var req updateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, errno.CodeInvalidParam, errno.Message(errno.CodeInvalidParam))
		return
	}

	result, err := h.studentService.UpdateStudentByNo(studentNo, service.UpdateStudentInput{
		Name:        req.Name,
		Phone:       req.Phone,
		Password:    req.Password,
		CreditLimit: req.CreditLimit,
		Status:      req.Status,
	})
	if err != nil {
		handleAdminStudentError(c, err)
		return
	}

	response.Success(c, result)
}

// handleAdminStudentError 统一处理学生管理相关错误。
func handleAdminStudentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrStudentNotFound):
		response.Fail(c, http.StatusNotFound, errno.CodeStudentNotFound, errno.Message(errno.CodeStudentNotFound))
	case errors.Is(err, service.ErrDuplicateStudentNo):
		response.Fail(c, http.StatusConflict, errno.CodeDuplicateStudentNo, errno.Message(errno.CodeDuplicateStudentNo))
	default:
		response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
	}
}
