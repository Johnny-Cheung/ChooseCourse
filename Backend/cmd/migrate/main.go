package main

import (
	"flag"
	"fmt"
	_ "time/tzdata"

	appconfig "choose-course-backend/internal/config"
	"choose-course-backend/internal/pkg/logger"
	"choose-course-backend/internal/repository"
)

// 这个入口程序专门负责“数据库准备工作”，不负责启动 HTTP 服务。
// 你可以把它理解成：
// 1. 先把表建好
// 2. 再把测试数据准备好
//
// 所以：
// - cmd/server 是“启动网站”
// - cmd/migrate 是“准备数据库”
//
// 为了降低理解成本，当前这里走的是最简单方案：
// 1. 用 Gorm 的 AutoMigrate 建表
// 2. 如果传了 -seed，就插入测试数据
//
// migrations/*.sql 和 scripts/seed.sql 目前保留为“手动参考脚本”，
// 不再作为程序自动执行路径的一部分。
func main() {
	// 这个命令行参数表示：
	// 执行迁移之后，是否顺便插入一批演示数据。
	//
	// 用法：
	// go run ./cmd/migrate
	// 只建表，不插初始数据
	//
	// go run ./cmd/migrate -seed
	// 建表 + 插入管理员/学生/课程测试数据
	withSeed := flag.Bool("seed", false, "seed initial data after migration")
	flag.Parse()

	// 第一步：加载配置。
	// 这里主要是为了拿到 MySQL 连接信息和日志配置。
	cfg, err := appconfig.Load("configs")
	if err != nil {
		panic(fmt.Errorf("load config failed: %w", err))
	}

	// 第二步：初始化日志。
	// 因为迁移过程中如果出错，我们希望看到清晰的日志信息。
	if err := logger.Init(cfg.Log); err != nil {
		panic(fmt.Errorf("init logger failed: %w", err))
	}
	defer logger.Sync()

	// 第三步：连接 MySQL。
	// 迁移只需要数据库，不需要 Redis，所以这里只初始化 MySQL。
	if err := repository.InitMySQL(cfg.MySQL); err != nil {
		logger.L().Fatal("init mysql failed", logger.Error(err))
	}
	defer repository.CloseMySQL()

	// 第四步：直接用 Gorm AutoMigrate 建表。
	// 这是当前项目里最容易理解的方式：
	// Gorm 会根据 internal/model 里的结构体，把表建出来。
	if err := repository.AutoMigrate(); err != nil {
		logger.L().Fatal("auto migrate failed", logger.Error(err))
	}
	logger.L().Info("auto migrate finished")

	// 第五步：如果传了 -seed，就额外插入初始数据。
	if *withSeed {
		if err := repository.SeedInitialData(); err != nil {
			logger.L().Fatal("seed initial data failed", logger.Error(err))
		}
		logger.L().Info("seed initial data finished")
	}
}
