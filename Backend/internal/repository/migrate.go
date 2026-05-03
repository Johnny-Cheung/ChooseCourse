package repository

import (
	"errors"

	"choose-course-backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// defaultPassword 是种子数据默认密码。
// 后续真正上线时，生产环境通常不会这样写死，
// 这里只是为了本地开发和测试方便。
const defaultPassword = "123456"

// AutoMigrate 使用 Gorm 根据模型自动建表和补索引。
//
// 这是当前项目里“最轻”的数据库准备方式。
// 你只要记住一句话：
// model 里写了什么结构，Gorm 就尽量帮你把表建成什么样。
//
// 现在的初学者版本只保留这一条主线，不再在代码里自动执行 SQL 文件。
func AutoMigrate() error {
	if mysqlDB == nil {
		return errors.New("mysql not initialized")
	}

	return mysqlDB.AutoMigrate(model.All()...)
}

// SeedInitialData 初始化一批演示数据。
// 这里使用 FirstOrCreate，重复执行不会重复插入同一批记录。
func SeedInitialData() error {
	if mysqlDB == nil {
		return errors.New("mysql not initialized")
	}

	// 这里把默认密码转成 bcrypt 哈希。
	// 数据库里绝不能存明文密码，所以哪怕是演示数据也要存哈希。
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 用事务包住整批种子数据写入。
	// 这样如果中间有一步失败，前面的插入也会一起回滚。
	return mysqlDB.Transaction(func(tx *gorm.DB) error {
		if err := seedAdmins(tx, string(passwordHash)); err != nil {
			return err
		}
		if err := seedStudents(tx, string(passwordHash)); err != nil {
			return err
		}
		if err := seedCourses(tx); err != nil {
			return err
		}

		return nil
	})
}

// seedAdmins 初始化管理员演示数据。
// FirstOrCreate 的意思是：
// - 如果数据库里没有这条记录，就插入
// - 如果已经有了，就直接拿现有记录，不重复插入
func seedAdmins(tx *gorm.DB, passwordHash string) error {
	admins := []model.Admin{
		{
			AdminNo:      "A0001",
			PasswordHash: passwordHash,
			Name:         "系统管理员",
			Phone:        "13800000001",
			Status:       1,
		},
	}

	for _, admin := range admins {
		if err := tx.Where("admin_no = ?", admin.AdminNo).FirstOrCreate(&admin).Error; err != nil {
			return err
		}
	}

	return nil
}

// seedStudents 初始化学生演示数据。
func seedStudents(tx *gorm.DB, passwordHash string) error {
	students := []model.Student{
		{
			StudentNo:    "20230001",
			PasswordHash: passwordHash,
			Name:         "张三",
			Phone:        "13800000011",
			CreditLimit:  25,
			CreditUsed:   0,
			Status:       1,
		},
		{
			StudentNo:    "20230002",
			PasswordHash: passwordHash,
			Name:         "李四",
			Phone:        "13800000012",
			CreditLimit:  25,
			CreditUsed:   0,
			Status:       1,
		},
		{
			StudentNo:    "20230003",
			PasswordHash: passwordHash,
			Name:         "王五",
			Phone:        "13800000013",
			CreditLimit:  25,
			CreditUsed:   0,
			Status:       1,
		},
	}

	for _, student := range students {
		if err := tx.Where("student_no = ?", student.StudentNo).FirstOrCreate(&student).Error; err != nil {
			return err
		}
	}

	return nil
}

// seedCourses 初始化课程演示数据。
// 这些课程主要用于后续课程查询、抢课、退课等接口联调。
func seedCourses(tx *gorm.DB) error {
	courses := []model.Course{
		{
			CourseName:    "高等数学",
			TeacherName:   "李老师",
			Capacity:      80,
			SelectedCount: 0,
			TimeSlot:      1,
			Credit:        4,
			Status:        1,
			LikeCount:     0,
			CommentCount:  0,
			Version:       1,
		},
		{
			CourseName:    "大学英语",
			TeacherName:   "王老师",
			Capacity:      60,
			SelectedCount: 0,
			TimeSlot:      2,
			Credit:        3,
			Status:        1,
			LikeCount:     0,
			CommentCount:  0,
			Version:       1,
		},
		{
			CourseName:    "数据结构",
			TeacherName:   "陈老师",
			Capacity:      50,
			SelectedCount: 0,
			TimeSlot:      5,
			Credit:        4,
			Status:        1,
			LikeCount:     0,
			CommentCount:  0,
			Version:       1,
		},
		{
			CourseName:    "体育",
			TeacherName:   "赵老师",
			Capacity:      100,
			SelectedCount: 0,
			TimeSlot:      8,
			Credit:        2,
			Status:        1,
			LikeCount:     0,
			CommentCount:  0,
			Version:       1,
		},
	}

	for _, course := range courses {
		// 这里复制一份局部变量，便于把它作为待插入数据传给 Gorm。
		record := course

		// 用“课程名 + 教师名 + 时间片”作为去重条件。
		// 这表示：同一个老师、同一个时间段、同名课程，不重复插入。
		if err := tx.Where("course_name = ? AND teacher_name = ? AND time_slot = ?", course.CourseName, course.TeacherName, course.TimeSlot).
			FirstOrCreate(&record).Error; err != nil {
			return err
		}
	}

	return nil
}
