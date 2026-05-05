package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// AuthUserState 表示认证中间件关心的“账号当前状态”。
//
// 这里只缓存最小必要信息：
// - 1: 账号存在且启用
// - 0: 账号存在但已禁用
// - -1: 账号不存在或已被删除
type AuthUserState int8

const (
	AuthUserStateMissing  AuthUserState = -1
	AuthUserStateDisabled AuthUserState = 0
	AuthUserStateActive   AuthUserState = 1

	authUserCacheBaseTTL = 9 * time.Minute
	authUserCacheJitter  = 3 * time.Minute
)

// AuthUserStateKey 返回某个认证账号状态缓存的 Redis key。
func AuthUserStateKey(role string, userID uint64) string {
	return fmt.Sprintf("auth:user:state:%s:%d", role, userID)
}

// GetAuthUserState 读取缓存中的账号状态。
//
// hit=false 表示缓存未命中；此时调用方应回源 MySQL。
func GetAuthUserState(ctx context.Context, role string, userID uint64) (state AuthUserState, hit bool, err error) {
	client, err := redisClient()
	if err != nil {
		return 0, false, err
	}

	value, err := client.Get(ctx, AuthUserStateKey(role, userID)).Result()
	if err != nil {
		if isRedisNil(err) {
			return 0, false, nil
		}
		return 0, false, err
	}

	parsed, err := strconv.ParseInt(value, 10, 8)
	if err != nil {
		// 非法值按 miss 处理，让调用方回源并回填正确数据。
		return 0, false, nil
	}

	state = AuthUserState(parsed)
	switch state {
	case AuthUserStateMissing, AuthUserStateDisabled, AuthUserStateActive:
		return state, true, nil
	default:
		// 未知值也按 miss 处理，避免脏缓存误导鉴权。
		return 0, false, nil
	}
}

// SetAuthUserState 回填账号状态缓存。
func SetAuthUserState(ctx context.Context, role string, userID uint64, state AuthUserState) error {
	client, err := redisClient()
	if err != nil {
		return err
	}

	return client.Set(ctx, AuthUserStateKey(role, userID), int(state), authUserStateTTL()).Err()
}

// InvalidateAuthUserState 删除账号状态缓存。
func InvalidateAuthUserState(ctx context.Context, role string, userID uint64) error {
	client, err := redisClient()
	if err != nil {
		return err
	}

	return client.Del(ctx, AuthUserStateKey(role, userID)).Err()
}

func authUserStateTTL() time.Duration {
	return ttlWithJitter(authUserCacheBaseTTL, authUserCacheJitter)
}
