package service

import (
	"choose-course-backend/internal/pkg/logger"
	"go.uber.org/zap"
)

// logPostCommitCacheSyncFailure 统一记录“数据库已提交，但缓存同步失败”的日志。
//
// 这类错误不应该再向上返回给前端，否则会出现：
// - 前端看到 500
// - MySQL 里的业务数据其实已经成功更新
//
// 所以这里的策略是：
// 1. 业务结果以 MySQL 提交成功为准
// 2. 缓存同步失败只记录日志，留给后续回源或补偿处理
func logPostCommitCacheSyncFailure(message string, err error, fields ...zap.Field) {
	logPostCommitSideEffectFailure(message, err, fields...)
}

// logPostCommitSideEffectFailure 统一记录“主事务成功后，附属异步动作失败”的日志。
//
// 例如：
// - 删除缓存失败
// - 发布通知消息失败
// - 其他已经不应该影响主业务结果的善后动作失败
func logPostCommitSideEffectFailure(message string, err error, fields ...zap.Field) {
	allFields := append([]zap.Field{logger.Error(err)}, fields...)
	logger.L().Error(message, allFields...)
}
