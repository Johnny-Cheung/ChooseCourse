package model

// Admin 对应管理员表。
//
// 管理员负责维护学生和课程数据，所以它本质上也是一种系统账号。
type Admin struct {
	SoftDeleteModel
	AdminNo      string `gorm:"column:admin_no;size:32;not null;uniqueIndex:uk_admin_no;comment:工号" json:"admin_no"` // 管理员工号，必须唯一
	PasswordHash string `gorm:"column:password_hash;size:255;not null;comment:密码摘要" json:"-"`                        // 管理员密码摘要，不返回给前端
	Name         string `gorm:"column:name;size:64;not null;comment:姓名" json:"name"`                                 // 管理员姓名
	Phone        string `gorm:"column:phone;size:20;not null;comment:手机号" json:"phone"`                              // 管理员手机号
	Status       int8   `gorm:"column:status;type:tinyint;not null;default:1;comment:状态 1正常 0禁用" json:"status"`      // 账号状态
}

// TableName 明确指定表名。
func (Admin) TableName() string {
	return "admins"
}
