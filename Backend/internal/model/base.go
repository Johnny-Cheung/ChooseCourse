package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 放所有表都会复用的公共字段。
//
// 你可以把它理解成“每张表的通用模板”。
// 只要一个业务模型嵌入了 BaseModel，它就自动拥有：
// - ID 主键
// - 创建时间
// - 更新时间
type BaseModel struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement;comment:主键ID" json:"id"` // 数据库主键，使用自增 ID
	CreatedAt time.Time `gorm:"not null;comment:创建时间" json:"created_at"`         // 记录首次创建的时间
	UpdatedAt time.Time `gorm:"not null;comment:更新时间" json:"updated_at"`         // 记录最近一次修改的时间
}

// SoftDeleteModel 给需要软删除的表复用。
//
// 所谓“软删除”就是：
// 不是真的把数据从数据库里删除掉，
// 而是把 deleted_at 填上时间，表示“这条数据逻辑上已经删除”。
//
// 这样做的好处是：
// - 后续还能追溯历史数据
// - 误删时更容易恢复
// - 某些业务需要保留审计痕迹
type SoftDeleteModel struct {
	BaseModel                // 先继承 ID、创建时间、更新时间
	DeletedAt gorm.DeletedAt `gorm:"index;comment:软删除时间" json:"-"` // 逻辑删除标记，接口返回时不暴露
}
