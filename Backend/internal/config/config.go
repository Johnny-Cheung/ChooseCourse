package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 是整个应用的总配置结构。
// 你可以把它理解成“config.yaml 在 Go 里的镜像”。
//
// config.yaml 里写了几段，这里就拆成几块：
// - app
// - log
// - mysql
// - redis
// - jwt
//
// Viper 读取 YAML 后，会按照 mapstructure 标签把值填到这些字段里。
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Log      LogConfig      `mapstructure:"log"`
	MySQL    MySQLConfig    `mapstructure:"mysql"`
	Redis    RedisConfig    `mapstructure:"redis"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	JWT      JWTConfig      `mapstructure:"jwt"`
}

// AppConfig 存放应用自身的运行配置。
// 这些配置不属于数据库或缓存，而是控制“这个服务本身怎么运行”。
type AppConfig struct {
	Name                   string `mapstructure:"name"`                     // 应用名，主要用于日志和后续服务标识
	Env                    string `mapstructure:"env"`                      // 当前环境，例如 dev/test/prod
	Port                   int    `mapstructure:"port"`                     // HTTP 服务监听端口
	ShutdownTimeoutSeconds int    `mapstructure:"shutdown_timeout_seconds"` // 优雅停机等待秒数
}

// LogConfig 控制日志级别和输出格式。
type LogConfig struct {
	Level    string `mapstructure:"level"`    // 日志级别，例如 info/debug
	Encoding string `mapstructure:"encoding"` // 输出格式，通常是 console 或 json
}

// MySQLConfig 存放数据库连接和连接池配置。
// 这部分配置决定程序如何连到 MySQL，以及一次最多保留多少连接。
type MySQLConfig struct {
	Host                   string `mapstructure:"host"`                      // 数据库主机地址
	Port                   int    `mapstructure:"port"`                      // 数据库端口
	User                   string `mapstructure:"user"`                      // 数据库用户名
	Password               string `mapstructure:"password"`                  // 数据库密码
	DBName                 string `mapstructure:"dbname"`                    // 要连接的数据库名
	Charset                string `mapstructure:"charset"`                   // 字符集，通常用 utf8mb4
	ParseTime              bool   `mapstructure:"parse_time"`                // 是否把 DATETIME 自动解析为 Go 的 time.Time
	Loc                    string `mapstructure:"loc"`                       // 时区设置
	MaxIdleConns           int    `mapstructure:"max_idle_conns"`            // 连接池中最多保留多少空闲连接
	MaxOpenConns           int    `mapstructure:"max_open_conns"`            // 连接池最多允许同时打开多少连接
	ConnMaxLifetimeMinutes int    `mapstructure:"conn_max_lifetime_minutes"` // 单个连接的最长复用时间
}

// RedisConfig 存放 Redis 连接和连接池配置。
type RedisConfig struct {
	Host         string `mapstructure:"host"`           // Redis 主机地址
	Port         int    `mapstructure:"port"`           // Redis 端口
	Password     string `mapstructure:"password"`       // Redis 密码
	DB           int    `mapstructure:"db"`             // 使用哪个逻辑库
	PoolSize     int    `mapstructure:"pool_size"`      // 连接池大小
	MinIdleConns int    `mapstructure:"min_idle_conns"` // 最少保留的空闲连接数
}

// RabbitMQConfig 存放 RabbitMQ 连接和队列拓扑配置。
//
// M7 里我们主要会用它来做：
// 1. 抢课请求异步投递
// 2. 后台消费者异步落库
type RabbitMQConfig struct {
	Host                       string `mapstructure:"host"`                          // RabbitMQ 主机地址
	Port                       int    `mapstructure:"port"`                          // RabbitMQ 端口，默认 5672
	User                       string `mapstructure:"user"`                          // RabbitMQ 用户名
	Password                   string `mapstructure:"password"`                      // RabbitMQ 密码
	VHost                      string `mapstructure:"vhost"`                         // RabbitMQ 虚拟主机，默认 /
	SelectionExchange          string `mapstructure:"selection_exchange"`            // 抢课消息主交换机
	GrabQueue                  string `mapstructure:"grab_queue"`                    // 抢课主队列基名；开启分片后会自动拼接分片号
	GrabRoutingKey             string `mapstructure:"grab_routing_key"`              // 抢课主路由键基名；开启分片后会自动拼接分片号
	SelectionShardCount        int    `mapstructure:"selection_shard_count"`         // 抢课队列分片数；同一课程会稳定路由到同一分片
	SelectionDeadExchange      string `mapstructure:"selection_dead_exchange"`       // 抢课死信交换机
	GrabDeadQueue              string `mapstructure:"grab_dead_queue"`               // 抢课死信队列名称
	GrabDeadRoutingKey         string `mapstructure:"grab_dead_routing_key"`         // 抢课死信路由键
	NotificationExchange       string `mapstructure:"notification_exchange"`         // 通知消息主交换机
	NotificationQueue          string `mapstructure:"notification_queue"`            // 通知主队列名称
	NotificationRoutingKey     string `mapstructure:"notification_routing_key"`      // 通知主路由键
	NotificationDeadExchange   string `mapstructure:"notification_dead_exchange"`    // 通知死信交换机
	NotificationDeadQueue      string `mapstructure:"notification_dead_queue"`       // 通知死信队列名称
	NotificationDeadRoutingKey string `mapstructure:"notification_dead_routing_key"` // 通知死信路由键
}

// JWTConfig 预留给后续登录鉴权使用。
// M0/M1 暂时还没真正用到，但配置先放好，M2 登录时会接上。
type JWTConfig struct {
	Secret      string `mapstructure:"secret"`       // JWT 签名密钥
	ExpireHours int    `mapstructure:"expire_hours"` // Token 过期时间（小时）
}

// Load 从指定目录读取 config.yaml，并映射成 Config 结构体。
//
// 这段代码做的事可以拆成 4 步：
// 1. 创建一个 Viper 实例
// 2. 告诉它去哪里找配置文件
// 3. 给一些配置设置默认值
// 4. 读取并反序列化到 Config 结构体
func Load(configDir string) (*Config, error) {
	v := viper.New()

	// 固定读取 config.yaml。
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir)

	// 允许用环境变量覆盖配置。
	// 例如：
	// app.port -> APP_PORT
	// mysql.host -> MYSQL_HOST
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 默认值在配置缺失时兜底，避免某些字段未写就直接报错。
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults 给常用配置设置默认值。
//
// 为什么需要默认值：
// - 开发时不必每次都把所有配置写全
// - 某些可选项缺失时，程序仍然能正常运行
// - 能减少“只是少写一个字段就启动失败”的问题
func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "choose-course")
	v.SetDefault("app.env", "dev")
	v.SetDefault("app.port", 8080)
	v.SetDefault("app.shutdown_timeout_seconds", 10)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.encoding", "console")

	v.SetDefault("mysql.charset", "utf8mb4")
	v.SetDefault("mysql.parse_time", true)
	v.SetDefault("mysql.loc", "Local")
	v.SetDefault("mysql.max_idle_conns", 10)
	v.SetDefault("mysql.max_open_conns", 100)
	v.SetDefault("mysql.conn_max_lifetime_minutes", 60)

	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 20)
	v.SetDefault("redis.min_idle_conns", 5)

	v.SetDefault("rabbitmq.port", 5672)
	v.SetDefault("rabbitmq.vhost", "/")
	v.SetDefault("rabbitmq.selection_exchange", "course.selection.exchange")
	v.SetDefault("rabbitmq.grab_queue", "course.selection.grab.queue")
	v.SetDefault("rabbitmq.grab_routing_key", "selection.grab")
	v.SetDefault("rabbitmq.selection_shard_count", 1)
	v.SetDefault("rabbitmq.selection_dead_exchange", "course.selection.dead.exchange")
	v.SetDefault("rabbitmq.grab_dead_queue", "course.selection.grab.dlq")
	v.SetDefault("rabbitmq.grab_dead_routing_key", "selection.grab.dead")
	v.SetDefault("rabbitmq.notification_exchange", "course.notification.exchange")
	v.SetDefault("rabbitmq.notification_queue", "course.notification.queue")
	v.SetDefault("rabbitmq.notification_routing_key", "notification.student")
	v.SetDefault("rabbitmq.notification_dead_exchange", "course.notification.dead.exchange")
	v.SetDefault("rabbitmq.notification_dead_queue", "course.notification.dlq")
	v.SetDefault("rabbitmq.notification_dead_routing_key", "notification.student.dead")

	v.SetDefault("jwt.expire_hours", 24)
}
