package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	appconfig "choose-course-backend/internal/config"
	"choose-course-backend/internal/mq"
	"choose-course-backend/internal/pkg/logger"
	"choose-course-backend/internal/repository"
	"choose-course-backend/internal/service"
)

func main() {
	// 第一步：读取配置文件。
	// worker 和 HTTP 服务共用同一套配置源。
	cfg, err := appconfig.Load("configs")
	if err != nil {
		panic(fmt.Errorf("load config failed: %w", err))
	}

	// 第二步：初始化日志。
	if err := logger.Init(cfg.Log); err != nil {
		panic(fmt.Errorf("init logger failed: %w", err))
	}
	defer logger.Sync()

	// 第三步：初始化 MySQL。
	// 异步消费者最终仍要以 MySQL 事务落库，所以数据库连接是必需的。
	if err := repository.InitMySQL(cfg.MySQL); err != nil {
		logger.L().Fatal("init mysql failed", logger.Error(err))
	}
	defer repository.CloseMySQL()

	// 第四步：初始化 Redis。
	// worker 失败补偿和 pending 超时收口都会依赖 Redis。
	if err := repository.InitRedis(cfg.Redis); err != nil {
		logger.L().Fatal("init redis failed", logger.Error(err))
	}
	defer repository.CloseRedis()

	// 第五步：初始化 RabbitMQ。
	if err := mq.Init(cfg.RabbitMQ); err != nil {
		logger.L().Fatal("init rabbitmq failed", logger.Error(err))
	}
	defer mq.Close()

	// 第六步：创建退出信号上下文。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	asyncSelectionService := service.NewSelectionAsyncService()
	notificationAsyncService := service.NewNotificationAsyncService()

	// 第七步：启动按课程分片的抢课消息消费者。
	if err := mq.StartSelectionGrabConsumers(ctx, asyncSelectionService.HandleGrabMessage); err != nil {
		logger.L().Fatal("start selection consumer failed", logger.Error(err))
	}

	// 第八步：启动通知消息消费者。
	if err := mq.StartNotificationConsumer(ctx, notificationAsyncService.HandleMessage); err != nil {
		logger.L().Fatal("start notification consumer failed", logger.Error(err))
	}

	// 第九步：启动 pending 超时收口器。
	go func() {
		ticker := time.NewTicker(asyncSelectionService.PendingSweepInterval())
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.L().Info("selection pending sweeper stopping")
				return
			case <-ticker.C:
				count, err := asyncSelectionService.SweepTimedOutPendingRequests(context.Background())
				if err != nil {
					logger.L().Error("sweep timed out pending requests failed", logger.Error(err))
					continue
				}
				if count > 0 {
					logger.L().Info("timed out pending requests swept", logger.Int("count", count))
				}
			}
		}
	}()

	logger.L().Info("background worker started", logger.String("env", cfg.App.Env))

	<-ctx.Done()
	logger.L().Info("selection worker shutdown signal received")
}
