package mq

import (
	"errors"
	"fmt"
)

var (
	// ErrDeadLetter 表示“这条消息不应该继续重试，而应该直接进入死信队列”。
	//
	// 它不是 RabbitMQ 自己抛出来的错误，而是我们业务代码主动打的标记。
	// 这样消费者拿到错误后，就能区分：
	// - 这是临时异常，应该继续重试
	// - 这是坏消息，应该直接送去 DLQ
	ErrDeadLetter = errors.New("dead letter")
)

// DeadLetter 把一个普通错误包装成“应该进死信队列”的错误。
func DeadLetter(err error) error {
	if err == nil {
		return ErrDeadLetter
	}

	return fmt.Errorf("%w: %v", ErrDeadLetter, err)
}
