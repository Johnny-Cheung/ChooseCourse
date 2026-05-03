package model

// Student 对应学生表。
//
// 这张表保存学生账号和学生基础资料。
// 后续登录、查个人信息、抢课资格判断都会依赖这张表。
type Student struct {
	SoftDeleteModel
	StudentNo    string `gorm:"column:student_no;size:32;not null;uniqueIndex:uk_student_no;comment:学号" json:"student_no"` // 学号，必须唯一，是学生最重要的业务标识
	PasswordHash string `gorm:"column:password_hash;size:255;not null;comment:密码摘要" json:"-"`                              // 存储加密后的密码摘要，不能返回给前端
	Name         string `gorm:"column:name;size:64;not null;comment:姓名" json:"name"`                                       // 学生姓名
	Phone        string `gorm:"column:phone;size:20;not null;index:idx_students_phone;comment:手机号" json:"phone"`           // 手机号，后续可用于联系和登录扩展
	CreditLimit  int    `gorm:"column:credit_limit;not null;default:25;comment:总学分上限" json:"credit_limit"`                 // 可选总学分上限，默认 25
	CreditUsed   int    `gorm:"column:credit_used;not null;default:0;comment:已用学分" json:"credit_used"`                     // 当前已经占用的学分，用于抢课时快速判断是否超限
	Status       int8   `gorm:"column:status;type:tinyint;not null;default:1;comment:状态 1正常 0禁用" json:"status"`            // 账号状态，禁用后可限制登录或选课
}

// TableName 明确告诉 Gorm：这个结构体映射到 students 表。
// 虽然 Gorm 也能自动推断，但手动指定更稳、更清晰。
func (Student) TableName() string {
	return "students"
}
