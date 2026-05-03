package repository

import (
	"fmt"
	"net/url"
	"time"

	appconfig "choose-course-backend/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// mysqlDB 保存全局 MySQL 连接实例。
var mysqlDB *gorm.DB

// InitMySQL 初始化 MySQL 连接，并配置连接池。
func InitMySQL(cfg appconfig.MySQLConfig) error {
	// DSN 是 MySQL 连接字符串，gorm.Open 会根据它建立连接。
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%t&loc=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.Charset,
		cfg.ParseTime,
		url.QueryEscape(cfg.Loc),
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}

	// gorm.DB 是 Gorm 封装后的对象，db.DB() 才能拿到底层 *sql.DB 并设置连接池。
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	// 这里设置数据库连接池参数，避免连接数失控或复用效率太低。
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeMinutes) * time.Minute)

	// 启动阶段主动 Ping，确保数据库真的连通。
	if err := sqlDB.Ping(); err != nil {
		return err
	}

	mysqlDB = db
	return nil
}

// DB 返回全局 Gorm 实例，后续 repository/service 会通过它访问数据库。
func DB() *gorm.DB {
	return mysqlDB
}

// PingMySQL 用于健康检查接口。
func PingMySQL() error {
	if mysqlDB == nil {
		return fmt.Errorf("mysql not initialized")
	}

	sqlDB, err := mysqlDB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Ping()
}

// CloseMySQL 在服务退出时关闭底层数据库连接。
func CloseMySQL() {
	if mysqlDB == nil {
		return
	}

	sqlDB, err := mysqlDB.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}
