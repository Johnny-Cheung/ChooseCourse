package service

import (
	"context"
	"errors"
	"time"

	"choose-course-backend/internal/cache"
	"choose-course-backend/internal/model"
	authjwt "choose-course-backend/internal/pkg/jwt"
	"choose-course-backend/internal/pkg/logger"
	"choose-course-backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	// ErrInvalidCredentials 表示账号不存在或密码不正确。
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserDisabled 表示账号已被禁用。
	ErrUserDisabled = errors.New("user disabled")
	// ErrUnsupportedRole 表示 token 里的角色不在系统支持范围内。
	ErrUnsupportedRole = errors.New("unsupported role")
)

// AuthService 专门处理认证相关业务。
// 当前它负责：
// 1. 学生登录
// 2. 管理员登录
// 3. 根据 token 获取当前登录用户信息
type AuthService struct{}

// LoginResult 是登录成功后返回给前端的数据。
type LoginResult struct {
	AccessToken string    `json:"access_token"` // 访问令牌，后续接口要带着它来访问
	TokenType   string    `json:"token_type"`   // 令牌类型，通常固定为 Bearer
	ExpiresAt   time.Time `json:"expires_at"`   // token 过期时间
	Role        string    `json:"role"`         // 当前用户角色
	Profile     any       `json:"profile"`      // 当前用户的基础资料
}

// StudentProfile 是学生身份下的返回资料。
type StudentProfile struct {
	ID          uint64 `json:"id"`
	StudentNo   string `json:"student_no"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	CreditLimit int    `json:"credit_limit"`
	CreditUsed  int    `json:"credit_used"`
	Status      int8   `json:"status"`
}

// AdminProfile 是管理员身份下的返回资料。
type AdminProfile struct {
	ID      uint64 `json:"id"`
	AdminNo string `json:"admin_no"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Status  int8   `json:"status"`
}

// MeResult 是 /auth/me 的统一返回结构。
type MeResult struct {
	Role    string `json:"role"`    // 当前登录用户角色
	Profile any    `json:"profile"` // 当前登录用户资料
}

// NewAuthService 创建一个认证服务实例。
func NewAuthService() *AuthService {
	return &AuthService{}
}

// StudentLogin 处理学生登录。
func (s *AuthService) StudentLogin(studentNo, password string) (*LoginResult, error) {
	var student model.Student
	if err := repository.DB().Where("student_no = ?", studentNo).First(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if student.Status != 1 {
		return nil, ErrUserDisabled
	}

	// 登录时永远使用密码哈希比较，不能自己把明文拿来直接比字符串。
	if err := bcrypt.CompareHashAndPassword([]byte(student.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, expiresAt, err := authjwt.GenerateToken(student.ID, authjwt.RoleStudent, student.StudentNo)
	if err != nil {
		return nil, err
	}

	preheatStudentLoginCaches(student)

	return &LoginResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		Role:        authjwt.RoleStudent,
		Profile: StudentProfile{
			ID:          student.ID,
			StudentNo:   student.StudentNo,
			Name:        student.Name,
			Phone:       student.Phone,
			CreditLimit: student.CreditLimit,
			CreditUsed:  student.CreditUsed,
			Status:      student.Status,
		},
	}, nil
}

// AdminLogin 处理管理员登录。
func (s *AuthService) AdminLogin(adminNo, password string) (*LoginResult, error) {
	var admin model.Admin
	if err := repository.DB().Where("admin_no = ?", adminNo).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if admin.Status != 1 {
		return nil, ErrUserDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, expiresAt, err := authjwt.GenerateToken(admin.ID, authjwt.RoleAdmin, admin.AdminNo)
	if err != nil {
		return nil, err
	}

	if err := cache.SetAuthUserState(context.Background(), authjwt.RoleAdmin, admin.ID, cache.AuthUserStateActive); err != nil {
		logPostCommitCacheSyncFailure(
			"post-login admin auth cache warm-up failed",
			err,
			logger.Any("admin_id", admin.ID),
			logger.String("admin_no", admin.AdminNo),
		)
	}

	return &LoginResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
		Role:        authjwt.RoleAdmin,
		Profile: AdminProfile{
			ID:      admin.ID,
			AdminNo: admin.AdminNo,
			Name:    admin.Name,
			Phone:   admin.Phone,
			Status:  admin.Status,
		},
	}, nil
}

// Me 根据 token 中的 Claims 查询当前登录用户资料。
func (s *AuthService) Me(claims *authjwt.Claims) (*MeResult, error) {
	switch claims.Role {
	case authjwt.RoleStudent:
		var student model.Student
		if err := repository.DB().First(&student, claims.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrInvalidCredentials
			}
			return nil, err
		}
		if student.Status != 1 {
			return nil, ErrUserDisabled
		}

		return &MeResult{
			Role: authjwt.RoleStudent,
			Profile: StudentProfile{
				ID:          student.ID,
				StudentNo:   student.StudentNo,
				Name:        student.Name,
				Phone:       student.Phone,
				CreditLimit: student.CreditLimit,
				CreditUsed:  student.CreditUsed,
				Status:      student.Status,
			},
		}, nil
	case authjwt.RoleAdmin:
		var admin model.Admin
		if err := repository.DB().First(&admin, claims.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrInvalidCredentials
			}
			return nil, err
		}
		if admin.Status != 1 {
			return nil, ErrUserDisabled
		}

		return &MeResult{
			Role: authjwt.RoleAdmin,
			Profile: AdminProfile{
				ID:      admin.ID,
				AdminNo: admin.AdminNo,
				Name:    admin.Name,
				Phone:   admin.Phone,
				Status:  admin.Status,
			},
		}, nil
	default:
		return nil, ErrUnsupportedRole
	}
}

func preheatStudentLoginCaches(student model.Student) {
	ctx := context.Background()

	if err := cache.SetAuthUserState(ctx, authjwt.RoleStudent, student.ID, cache.AuthUserStateActive); err != nil {
		logPostCommitCacheSyncFailure(
			"post-login student auth cache warm-up failed",
			err,
			logger.Any("student_id", student.ID),
			logger.String("student_no", student.StudentNo),
		)
	}

	if err := cache.EnsureStudentSelectionCache(ctx, student.ID); err != nil {
		logPostCommitCacheSyncFailure(
			"post-login student selection cache warm-up failed",
			err,
			logger.Any("student_id", student.ID),
			logger.String("student_no", student.StudentNo),
		)
	}
}
