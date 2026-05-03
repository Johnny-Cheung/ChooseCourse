package service

import (
	"context"
	"errors"

	"choose-course-backend/internal/cache"
	"choose-course-backend/internal/model"
	authjwt "choose-course-backend/internal/pkg/jwt"
	"choose-course-backend/internal/pkg/logger"
	"choose-course-backend/internal/repository"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	// ErrStudentNotFound 表示按学号没有找到学生。
	ErrStudentNotFound = errors.New("student not found")
	// ErrDuplicateStudentNo 表示新增学生时学号重复。
	ErrDuplicateStudentNo = errors.New("duplicate student no")
)

// CreateStudentInput 是管理员新增学生时提交的数据。
type CreateStudentInput struct {
	StudentNo   string // 学号，必填，必须唯一
	Password    string // 初始密码，必填
	Name        string // 学生姓名，必填
	Phone       string // 手机号，必填
	CreditLimit int    // 总学分上限，默认 25
	Status      int8   // 状态，默认 1
}

// UpdateStudentInput 是管理员修改学生时允许提交的数据。
// 学号不允许在这里修改，所以没有 StudentNo 字段。
type UpdateStudentInput struct {
	Name        *string // 管理员可以帮学生改姓名
	Phone       *string // 管理员可以帮学生改手机号
	Password    *string // 管理员可以重置学生密码
	CreditLimit *int    // 管理员可以调整学分上限
	Status      *int8   // 管理员可以启用/禁用学生
}

// StudentDetail 是管理端查看学生详情时返回的数据。
type StudentDetail struct {
	ID          uint64 `json:"id"`
	StudentNo   string `json:"student_no"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	CreditLimit int    `json:"credit_limit"`
	CreditUsed  int    `json:"credit_used"`
	Status      int8   `json:"status"`
}

// AdminStudentService 负责管理员侧的学生管理。
type AdminStudentService struct{}

// NewAdminStudentService 创建学生管理服务。
func NewAdminStudentService() *AdminStudentService {
	return &AdminStudentService{}
}

// CreateStudent 新增学生。
func (s *AdminStudentService) CreateStudent(input CreateStudentInput) (*StudentDetail, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 如果调用方没显式传学分上限，就使用默认值 25。
	creditLimit := input.CreditLimit
	if creditLimit <= 0 {
		creditLimit = 25
	}

	// 如果调用方没显式传状态，就默认是正常状态。
	status := input.Status
	if status != 0 && status != 1 {
		status = 1
	}

	student := model.Student{
		StudentNo:    input.StudentNo,
		PasswordHash: string(passwordHash),
		Name:         input.Name,
		Phone:        input.Phone,
		CreditLimit:  creditLimit,
		CreditUsed:   0,
		Status:       status,
	}

	if err := repository.DB().Create(&student).Error; err != nil {
		if isDuplicateEntry(err) {
			return nil, ErrDuplicateStudentNo
		}

		return nil, err
	}

	return &StudentDetail{
		ID:          student.ID,
		StudentNo:   student.StudentNo,
		Name:        student.Name,
		Phone:       student.Phone,
		CreditLimit: student.CreditLimit,
		CreditUsed:  student.CreditUsed,
		Status:      student.Status,
	}, nil
}

// GetStudentByNo 根据学号查询学生。
func (s *AdminStudentService) GetStudentByNo(studentNo string) (*StudentDetail, error) {
	var student model.Student
	if err := repository.DB().Where("student_no = ?", studentNo).First(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStudentNotFound
		}

		return nil, err
	}

	return &StudentDetail{
		ID:          student.ID,
		StudentNo:   student.StudentNo,
		Name:        student.Name,
		Phone:       student.Phone,
		CreditLimit: student.CreditLimit,
		CreditUsed:  student.CreditUsed,
		Status:      student.Status,
	}, nil
}

// UpdateStudentByNo 按学号修改学生信息。
func (s *AdminStudentService) UpdateStudentByNo(studentNo string, input UpdateStudentInput) (*StudentDetail, error) {
	var student model.Student
	if err := repository.DB().Where("student_no = ?", studentNo).First(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStudentNotFound
		}

		return nil, err
	}

	updates := make(map[string]any)

	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Phone != nil {
		updates["phone"] = *input.Phone
	}
	if input.CreditLimit != nil {
		updates["credit_limit"] = *input.CreditLimit
	}
	if input.Status != nil {
		updates["status"] = *input.Status
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

		// M6 之后，抢课前置校验会依赖学生自己的 Redis 缓存，
		// 比如学分上限、已用学分、已选课程时间位图等。
		//
		// 所以管理员修改学生信息后，最稳妥的做法是删掉这个学生的缓存，
		// 让后续真正发起抢课请求时再从 MySQL 重建。
		//
		// 这里即使这次改动只是 name/phone/password，
		// 我们也统一删除学生缓存，原因是：
		// - 实现更简单
		// - 不容易漏掉以后新增的“会影响抢课判断”的字段
		if err := cache.InvalidateStudentSelectionCache(context.Background(), student.ID); err != nil {
			logPostCommitCacheSyncFailure(
				"post-commit student cache invalidation failed",
				err,
				logger.Any("student_id", student.ID),
				logger.String("student_no", student.StudentNo),
			)
		}
		if err := cache.InvalidateAuthUserState(context.Background(), authjwt.RoleStudent, student.ID); err != nil {
			logPostCommitCacheSyncFailure(
				"post-commit student auth cache invalidation failed",
				err,
				logger.Any("student_id", student.ID),
				logger.String("student_no", student.StudentNo),
			)
		}
	}

	return s.GetStudentByNo(studentNo)
}

// isDuplicateEntry 用来识别 MySQL 的唯一索引冲突错误。
func isDuplicateEntry(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}

	return false
}
