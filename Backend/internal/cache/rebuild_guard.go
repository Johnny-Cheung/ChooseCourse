package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrCourseSelectionCacheNotFound  = errors.New("course selection cache source not found")
	ErrStudentSelectionCacheNotFound = errors.New("student selection cache source not found")
	ErrSelectionCacheRebuildBusy     = errors.New("selection cache rebuild busy")
)

const (
	selectionCacheRebuildLockTTL     = 3 * time.Second
	selectionCacheRebuildWaitTimeout = 800 * time.Millisecond
	selectionCacheRebuildWaitStep    = 40 * time.Millisecond

	selectionMissingCacheBaseTTL = 30 * time.Second
	selectionMissingCacheJitter  = 30 * time.Second
)

var releaseSelectionCacheRebuildLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

func acquireSelectionCacheRebuildLock(ctx context.Context, lockKey string) (string, bool, error) {
	client, err := redisClient()
	if err != nil {
		return "", false, err
	}

	token, err := newLockToken()
	if err != nil {
		return "", false, err
	}

	locked, err := client.SetNX(ctx, lockKey, token, selectionCacheRebuildLockTTL).Result()
	if err != nil {
		return "", false, err
	}

	return token, locked, nil
}

func releaseSelectionCacheRebuildLock(ctx context.Context, lockKey, token string) {
	client, err := redisClient()
	if err != nil {
		return
	}

	_, _ = releaseSelectionCacheRebuildLockScript.Run(ctx, client, []string{lockKey}, token).Result()
}

func waitForSelectionCacheReady(ctx context.Context, ready func(context.Context) (bool, error), missing func(context.Context) (bool, error), notFoundErr error) error {
	timer := time.NewTimer(selectionCacheRebuildWaitTimeout)
	defer timer.Stop()

	ticker := time.NewTicker(selectionCacheRebuildWaitStep)
	defer ticker.Stop()

	for {
		if exists, err := missing(ctx); err != nil {
			return err
		} else if exists {
			return notFoundErr
		}

		if ok, err := ready(ctx); err != nil {
			return err
		} else if ok {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return ErrSelectionCacheRebuildBusy
		case <-ticker.C:
		}
	}
}

func ttlWithJitter(base, jitter time.Duration) time.Duration {
	if jitter <= 0 {
		return base
	}

	return base + time.Duration(time.Now().UnixNano()%int64(jitter))
}

func selectionMissingCacheTTL() time.Duration {
	return ttlWithJitter(selectionMissingCacheBaseTTL, selectionMissingCacheJitter)
}

func newLockToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf[:]), nil
}
