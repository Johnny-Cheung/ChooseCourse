package service

import (
	"errors"

	"choose-course-backend/internal/model"
	"choose-course-backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UpdateStudentProfileInput 表示学生修改个人资料时允许提交的字段。
// 学号和姓名不允许在这里修改，所以这里只保留手机号和密码。
type UpdateStudentProfileInput struct {
	Phone    *string // 学生手机号，可选
	Password *string // 学生新密码，可选
}

// StudentProfileService 负责学生“查看和修改个人资料”。
type StudentProfileService struct{}

// NewStudentProfileService 创建学生个人资料服务。
func NewStudentProfileService() *StudentProfileService {
	return &StudentProfileService{}
}

// GetProfile 根据学生 ID 查询学生资料。
func (s *StudentProfileService) GetProfile(studentID uint64) (*StudentProfile, error) {
	var student model.Student
	if err := repository.DB().First(&student, studentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStudentNotFound
		}

		return nil, err
	}

	return &StudentProfile{
		ID:          student.ID,
		StudentNo:   student.StudentNo,
		Name:        student.Name,
		Phone:       student.Phone,
		CreditLimit: student.CreditLimit,
		CreditUsed:  student.CreditUsed,
		Status:      student.Status,
	}, nil
}

// UpdateProfile 修改学生自己的资料。
// 当前只允许改：
// - phone
// - password
func (s *StudentProfileService) UpdateProfile(studentID uint64, input UpdateStudentProfileInput) (*StudentProfile, error) {
	var student model.Student
	if err := repository.DB().First(&student, studentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStudentNotFound
		}

		return nil, err
	}

	updates := make(map[string]any)

	if input.Phone != nil {
		updates["phone"] = *input.Phone
	}
	if input.Password != nil {
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		updates["password_hash"] = string(passwordHash)
	}

	if len(updates) > 0 {
		if err := repository.DB().Model(&student).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return s.GetProfile(studentID)
}
