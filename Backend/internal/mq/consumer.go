package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"choose-course-backend/internal/pkg/logger"
)

// SelectionGrabHandler 是“抢课消息消费者”真正要执行的业务函数签名。
type SelectionGrabHandler func(context.Context, SelectionMessage) error

// NotificationHandler 是“通知消息消费者”真正要执行的业务函数签名。
type NotificationHandler func(context.Context, NotificationMessage) error

// StartSelectionGrabConsumers 启动全部“抢课消息”分片消费者。
//
// 同一门课会稳定落到同一个 queue shard，因此：
// - 单课程内仍然是串行消费
// - 多门课程可以分散到不同 shard 并行落库
func StartSelectionGrabConsumers(ctx context.Context, handler SelectionGrabHandler) error {
	for shard := 0; shard < selectionGrabShardCount(); shard++ {
		if err := startSelectionGrabConsumerForShard(ctx, handler, shard); err != nil {
			return err
		}
	}

	return nil
}

// StartSelectionGrabConsumer 保留给旧调用方的兼容入口。
func StartSelectionGrabConsumer(ctx context.Context, handler SelectionGrabHandler) error {
	return StartSelectionGrabConsumers(ctx, handler)
}

// startSelectionGrabConsumerForShard 启动单个分片的抢课消费者。
func startSelectionGrabConsumerForShard(ctx context.Context, handler SelectionGrabHandler, shard int) error {
	if conn == nil {
		return fmt.Errorf("rabbitmq not initialized")
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	if err := declareSelectionGrabShardTopologyWithChannel(ch, shard); err != nil {
		_ = ch.Close()
		return err
	}

	queueName := selectionGrabQueueName(shard)

	deliveries, err := ch.Consume(
		queueName,
		fmt.Sprintf("selection-grab-shard-%d", shard),
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = ch.Close()
		return err
	}

	go func() {
		defer ch.Close()

		for {
			select {
			case <-ctx.Done():
				logger.L().Info("selection grab consumer stopping", logger.Int("shard", shard), logger.String("queue", queueName))
				return
			case delivery, ok := <-deliveries:
				if !ok {
					logger.L().Info("selection grab consumer channel closed", logger.Int("shard", shard), logger.String("queue", queueName))
					return
				}

				var message SelectionMessage
				if err := json.Unmarshal(delivery.Body, &message); err != nil {
					logger.L().Error("unmarshal selection message failed", logger.Error(err), logger.Int("shard", shard))
					// 消息体都解析不了，说明这是一条坏消息。
					// 这种情况重试没有意义，直接送去死信队列。
					_ = delivery.Nack(false, false)
					continue
				}

				if err := handler(ctx, message); err != nil {
					logger.L().Error(
						"handle selection message failed",
						logger.Error(err),
						logger.Int("shard", shard),
						logger.String("request_no", message.RequestNo),
					)

					// 如果业务代码明确把错误标记成“应该进死信队列”，
					// 那就不要再反复重试了，直接 Nack(false, false) 交给 DLQ。
					if errors.Is(err, ErrDeadLetter) {
						_ = delivery.Nack(false, false)
						continue
					}

					// 其余错误默认先按“临时异常”处理，继续重试。
					_ = delivery.Nack(false, true)
					continue
				}

				_ = delivery.Ack(false)
			}
		}
	}()

	return nil
}

// StartNotificationConsumer 启动“通知消息”消费者。
func StartNotificationConsumer(ctx context.Context, handler NotificationHandler) error {
	if conn == nil {
		return fmt.Errorf("rabbitmq not initialized")
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	if err := declareNotificationTopologyWithChannel(ch); err != nil {
		_ = ch.Close()
		return err
	}

	deliveries, err := ch.Consume(
		rabbitCfg.NotificationQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = ch.Close()
		return err
	}

	go func() {
		defer ch.Close()

		for {
			select {
			case <-ctx.Done():
				logger.L().Info("notification consumer stopping")
				return
			case delivery, ok := <-deliveries:
				if !ok {
					logger.L().Info("notification consumer channel closed")
					return
				}

				var message NotificationMessage
				if err := json.Unmarshal(delivery.Body, &message); err != nil {
					logger.L().Error("unmarshal notification message failed", logger.Error(err))
					_ = delivery.Nack(false, false)
					continue
				}

				if err := handler(ctx, message); err != nil {
					logger.L().Error(
						"handle notification message failed",
						logger.Error(err),
						logger.String("biz_key", message.BizKey),
						logger.String("biz_type", message.BizType),
					)

					if errors.Is(err, ErrDeadLetter) {
						_ = delivery.Nack(false, false)
						continue
					}

					_ = delivery.Nack(false, true)
					continue
				}

				_ = delivery.Ack(false)
			}
		}
	}()

	return nil
}
