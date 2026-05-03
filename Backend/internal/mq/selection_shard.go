package mq

import "fmt"

// selectionGrabShardCount 返回当前启用的抢课分片数。
//
// 这里统一做兜底，避免配置写成 0 或负数时让后续拓扑计算失效。
func selectionGrabShardCount() int {
	if rabbitCfg.SelectionShardCount <= 0 {
		return 1
	}

	return rabbitCfg.SelectionShardCount
}

// selectionGrabShardForCourse 根据 courseID 计算这门课应该落到哪个分片。
//
// 这样可以保证：
// - 同一门课永远进入同一个 queue shard
// - 不同课程可以分散到不同 shard 并行消费
func selectionGrabShardForCourse(courseID uint64) int {
	return int(courseID % uint64(selectionGrabShardCount()))
}

// selectionGrabQueueName 返回某个分片对应的主抢课队列名。
func selectionGrabQueueName(shard int) string {
	if selectionGrabShardCount() == 1 {
		return rabbitCfg.GrabQueue
	}

	return fmt.Sprintf("%s.%d", rabbitCfg.GrabQueue, shard)
}

// selectionGrabRoutingKey 返回某个分片对应的主路由键。
func selectionGrabRoutingKey(shard int) string {
	if selectionGrabShardCount() == 1 {
		return rabbitCfg.GrabRoutingKey
	}

	return fmt.Sprintf("%s.%d", rabbitCfg.GrabRoutingKey, shard)
}
