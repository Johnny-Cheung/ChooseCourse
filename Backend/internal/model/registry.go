package model

// All 返回需要迁移的全部模型。
//
// 这样 repository 层只需要调用一次：
// AutoMigrate(model.All()...)
//
// 就能把所有表一次性迁移掉。
// 以后新增模型时，只要把它补进这里即可。
func All() []any {
	return []any{
		&Student{},
		&Admin{},
		&Course{},
		&Enrollment{},
		&CourseLike{},
		&CourseComment{},
		&Notification{},
		&SelectionRequest{},
	}
}
