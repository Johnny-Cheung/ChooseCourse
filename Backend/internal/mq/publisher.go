package mq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// PublishSelectionGrab 发布一条“抢课请求”消息。
//
// 为什么发布时每次都新建 channel：
// RabbitMQ 的 Channel 不适合在高并发下被多个 goroutine 随意共享，
// 对当前这个项目来说，“每次发布开一个 channel”更简单也更安全。
func PublishSelectionGrab(ctx context.Context, message SelectionMessage) error {
	if conn == nil {
		return fmt.Errorf("rabbitmq not initialized")
	}

	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	shard := selectionGrabShardForCourse(message.CourseID)

	// 再声明一次目标分片拓扑是安全的。
	// RabbitMQ 的声明操作是幂等的，这样即使启动顺序变化，也能保证发布前基础资源存在。
	if err := declareSelectionGrabShardTopologyWithChannel(ch, shard); err != nil {
		return err
	}

	return ch.PublishWithContext(
		ctx,
		rabbitCfg.SelectionExchange,
		selectionGrabRoutingKey(shard),
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

// PublishNotification 发布一条“异步通知”消息。
func PublishNotification(ctx context.Context, message NotificationMessage) error {
	if conn == nil {
		return fmt.Errorf("rabbitmq not initialized")
	}

	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := declareNotificationTopologyWithChannel(ch); err != nil {
		return err
	}

	return ch.PublishWithContext(
		ctx,
		rabbitCfg.NotificationExchange,
		rabbitCfg.NotificationRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}
