package service

import (
	"errors"

	"choose-course-backend/internal/model"
	"choose-course-backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	// ErrAdminNotFound 表示管理员不存在。
	ErrAdminNotFound = errors.New("admin not found")
)

// UpdateAdminProfileInput 表示管理员修改个人信息时允许提交的字段。
// 这里用指针字段是为了区分：
// - 没传这个字段
// - 传了，但值为空字符串
type UpdateAdminProfileInput struct {
	Name     *string // 管理员姓名，可修改
	Phone    *string // 管理员手机号，可修改
	Password *string // 管理员密码，可修改
}

// AdminService 负责管理员自身资料相关业务。
type AdminService struct{}

// NewAdminService 创建管理员服务。
func NewAdminService() *AdminService {
	return &AdminService{}
}

// GetProfile 根据管理员 ID 查询个人资料。
func (s *AdminService) GetProfile(adminID uint64) (*AdminProfile, error) {
	var admin model.Admin
	if err := repository.DB().First(&admin, adminID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAdminNotFound
		}

		return nil, err
	}

	return &AdminProfile{
		ID:      admin.ID,
		AdminNo: admin.AdminNo,
		Name:    admin.Name,
		Phone:   admin.Phone,
		Status:  admin.Status,
	}, nil
}

// UpdateProfile 修改管理员个人信息。
// 当前允许修改：
// - name
// - phone
// - password
func (s *AdminService) UpdateProfile(adminID uint64, input UpdateAdminProfileInput) (*AdminProfile, error) {
	var admin model.Admin
	if err := repository.DB().First(&admin, adminID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAdminNotFound
		}

		return nil, err
	}

	updates := make(map[string]any)

	// 只有调用方真的传了字段时，才把它加入更新集合。
	if input.Name != nil {
		updates["name"] = *input.Name
	}
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

	// 如果一个字段都没传，就直接返回当前资料，不必执行数据库更新。
	if len(updates) > 0 {
		if err := repository.DB().Model(&admin).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return s.GetProfile(adminID)
}
