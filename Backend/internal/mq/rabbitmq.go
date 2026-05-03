package mq

import (
	"fmt"
	"net/url"

	appconfig "choose-course-backend/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	// conn 保存全局 RabbitMQ 连接。
	// 当前项目和 MySQL/Redis 一样，先采用“全局连接”的最简单方案。
	conn *amqp.Connection

	// rabbitCfg 保存 RabbitMQ 配置。
	// publisher / consumer / topology 声明都会用到它。
	rabbitCfg appconfig.RabbitMQConfig
)

// Init 建立 RabbitMQ 连接，并提前声明 M7 需要的交换机和队列。
func Init(cfg appconfig.RabbitMQConfig) error {
	dsn := fmt.Sprintf(
		"amqp://%s:%s@%s:%d/%s",
		url.QueryEscape(cfg.User),
		url.QueryEscape(cfg.Password),
		cfg.Host,
		cfg.Port,
		url.PathEscape(cfg.VHost),
	)

	openedConn, err := amqp.Dial(dsn)
	if err != nil {
		return err
	}

	conn = openedConn
	rabbitCfg = cfg

	// 先把本项目需要的基础拓扑声明好。
	if err := declareSelectionGrabTopology(); err != nil {
		_ = conn.Close()
		conn = nil
		return err
	}
	if err := declareNotificationTopology(); err != nil {
		_ = conn.Close()
		conn = nil
		return err
	}

	return nil
}

// Connection 返回全局 RabbitMQ 连接。
func Connection() *amqp.Connection {
	return conn
}

// Config 返回当前 RabbitMQ 配置。
func Config() appconfig.RabbitMQConfig {
	return rabbitCfg
}

// Close 关闭 RabbitMQ 全局连接。
func Close() {
	if conn != nil {
		_ = conn.Close()
	}
}

// PingRabbitMQ 用于健康检查。
//
// 这里只做一件最小但有效的事：
// 尝试基于当前连接创建一个临时 channel。
// 如果能成功创建并关闭，说明 RabbitMQ 连接当前仍可用。
func PingRabbitMQ() error {
	if conn == nil {
		return fmt.Errorf("rabbitmq not initialized")
	}
	if conn.IsClosed() {
		return fmt.Errorf("rabbitmq connection closed")
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return nil
}

// declareSelectionGrabTopology 声明抢课异步流程需要的全部分片拓扑。
//
// 当前策略是：
// - 一个主 exchange
// - 一个共享死信 exchange
// - N 个按课程分片的主 grab queue
// - 一个共享死信队列
func declareSelectionGrabTopology() error {
	if conn == nil {
		return fmt.Errorf("rabbitmq not initialized")
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return declareSelectionGrabTopologyWithChannel(ch)
}

// declareSelectionGrabTopologyWithChannel 用指定 channel 声明全部抢课分片拓扑。
func declareSelectionGrabTopologyWithChannel(ch *amqp.Channel) error {
	if err := declareSelectionGrabExchangesWithChannel(ch); err != nil {
		return err
	}

	for shard := 0; shard < selectionGrabShardCount(); shard++ {
		if err := declareSelectionGrabShardTopologyWithChannel(ch, shard); err != nil {
			return err
		}
	}

	return nil
}

// declareSelectionGrabExchangesWithChannel 声明抢课流程使用的交换机和共享死信队列。
func declareSelectionGrabExchangesWithChannel(ch *amqp.Channel) error {
	// 先声明主交换机。
	if err := ch.ExchangeDeclare(
		rabbitCfg.SelectionExchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	// 再声明死信交换机。
	if err := ch.ExchangeDeclare(
		rabbitCfg.SelectionDeadExchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	// 当前所有抢课分片共用一个死信队列，便于集中排查坏消息。
	if _, err := ch.QueueDeclare(
		rabbitCfg.GrabDeadQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	return ch.QueueBind(
		rabbitCfg.GrabDeadQueue,
		rabbitCfg.GrabDeadRoutingKey,
		rabbitCfg.SelectionDeadExchange,
		false,
		nil,
	)
}

// declareSelectionGrabShardTopologyWithChannel 声明一个具体分片的主队列和绑定关系。
func declareSelectionGrabShardTopologyWithChannel(ch *amqp.Channel, shard int) error {
	if err := declareSelectionGrabExchangesWithChannel(ch); err != nil {
		return err
	}

	queueName := selectionGrabQueueName(shard)
	routingKey := selectionGrabRoutingKey(shard)

	// 主抢课队列上挂共享死信交换机。
	// 这样当消息被 Nack(requeue=false) 或 Reject(requeue=false) 时，
	// RabbitMQ 会自动把它转发到死信交换机，再路由到共享死信队列。
	if _, err := ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    rabbitCfg.SelectionDeadExchange,
			"x-dead-letter-routing-key": rabbitCfg.GrabDeadRoutingKey,
		},
	); err != nil {
		return err
	}

	return ch.QueueBind(
		queueName,
		routingKey,
		rabbitCfg.SelectionExchange,
		false,
		nil,
	)
}

// declareNotificationTopology 声明通知异步写入流程需要的 exchange / queue / binding。
func declareNotificationTopology() error {
	if conn == nil {
		return fmt.Errorf("rabbitmq not initialized")
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return declareNotificationTopologyWithChannel(ch)
}

// declareNotificationTopologyWithChannel 用指定 channel 声明通知相关拓扑。
func declareNotificationTopologyWithChannel(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(
		rabbitCfg.NotificationExchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	if err := ch.ExchangeDeclare(
		rabbitCfg.NotificationDeadExchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	if _, err := ch.QueueDeclare(
		rabbitCfg.NotificationQueue,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    rabbitCfg.NotificationDeadExchange,
			"x-dead-letter-routing-key": rabbitCfg.NotificationDeadRoutingKey,
		},
	); err != nil {
		return err
	}

	if _, err := ch.QueueDeclare(
		rabbitCfg.NotificationDeadQueue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	if err := ch.QueueBind(
		rabbitCfg.NotificationQueue,
		rabbitCfg.NotificationRoutingKey,
		rabbitCfg.NotificationExchange,
		false,
		nil,
	); err != nil {
		return err
	}

	return ch.QueueBind(
		rabbitCfg.NotificationDeadQueue,
		rabbitCfg.NotificationDeadRoutingKey,
		rabbitCfg.NotificationDeadExchange,
		false,
		nil,
	)
}
