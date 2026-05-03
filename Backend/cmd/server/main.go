package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"choose-course-backend/internal/cache"
	appconfig "choose-course-backend/internal/config"
	"choose-course-backend/internal/mq"
	authjwt "choose-course-backend/internal/pkg/jwt"
	"choose-course-backend/internal/pkg/logger"
	"choose-course-backend/internal/repository"
	"choose-course-backend/internal/router"
)

func main() {
	// 第一步：读取配置文件。
	// 这里会从 configs/config.yaml 中读取应用端口、数据库、Redis 等信息。
	cfg, err := appconfig.Load("configs")
	if err != nil {
		panic(fmt.Errorf("load config failed: %w", err))
	}

	// 第二步：初始化日志组件。
	// 后面所有启动日志、请求日志、错误日志都依赖这个 logger。
	if err := logger.Init(cfg.Log); err != nil {
		panic(fmt.Errorf("init logger failed: %w", err))
	}
	defer logger.Sync()

	// 第三步：初始化 JWT 组件。
	// 后续登录接口签发 token、鉴权中间件校验 token 都依赖这里的配置。
	if err := authjwt.Init(cfg.JWT); err != nil {
		logger.L().Fatal("init jwt failed", logger.Error(err))
	}

	// 第四步：初始化 MySQL 连接。
	// 程序启动时会主动 Ping 一次数据库，确保配置和连接都是可用的。
	if err := repository.InitMySQL(cfg.MySQL); err != nil {
		logger.L().Fatal("init mysql failed", logger.Error(err))
	}
	defer repository.CloseMySQL()

	// 第五步：初始化 Redis 连接。
	// 同样会在启动时做一次 Ping，避免服务启动后才发现缓存不可用。
	if err := repository.InitRedis(cfg.Redis); err != nil {
		logger.L().Fatal("init redis failed", logger.Error(err))
	}
	defer repository.CloseRedis()

	// 第六步：启动时批量预热选课缓存。
	// 这里会：
	// 1. 预热所有开课课程的选课缓存
	// 2. 删除不开课课程残留的旧选课缓存
	// 3. 预热所有学生的选课缓存
	preloadStartedAt := time.Now()
	preloadResult, err := cache.PreloadSelectionCaches(context.Background())
	if err != nil {
		logger.L().Error("preload selection caches failed", logger.Error(err))
	} else {
		logger.L().Info(
			"selection caches preloaded",
			logger.Int("open_courses_loaded", preloadResult.OpenCoursesLoaded),
			logger.Int("closed_courses_invalidated", preloadResult.ClosedCoursesInvalidated),
			logger.Int("students_loaded", preloadResult.StudentsLoaded),
			logger.Duration("latency", time.Since(preloadStartedAt)),
		)
	}

	// 第七步：初始化 RabbitMQ。
	// M7 开始，抢课接口会依赖 RabbitMQ 做异步投递和消费。
	if err := mq.Init(cfg.RabbitMQ); err != nil {
		logger.L().Fatal("init rabbitmq failed", logger.Error(err))
	}
	defer mq.Close()

	// 第八步：提前创建一个“接收系统退出信号”的上下文。
	// API 服务现在只负责受理请求和发布 MQ，不再在同进程里消费消息。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 第九步：创建 Gin 路由引擎，并交给 http.Server 托管。
	// router.New() 内部会注册中间件和基础路由。
	engine := router.New()
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 第十步：在单独 goroutine 中启动 HTTP 服务。
	// main goroutine 会继续往下执行，用于监听退出信号。
	go func() {
		logger.L().Info("http server starting", logger.String("addr", addr), logger.String("env", cfg.App.Env))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.L().Fatal("http server stopped unexpectedly", logger.Error(err))
		}
	}()

	<-ctx.Done()
	logger.L().Info("shutdown signal received")

	// 第十一步：给优雅停机设置超时时间。
	// 这段时间内，服务会停止接收新请求，并尽量等正在执行的请求完成。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.App.ShutdownTimeoutSeconds)*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.L().Error("graceful shutdown failed", logger.Error(err))
	}
}
