package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const selectionRequestPendingDeadlineKey = "selection:request:pending_deadline"

// SelectionRequestState is the short-lived Redis view of an async selection
// request. MySQL remains the final audit store after the worker finishes.
type SelectionRequestState struct {
	RequestNo  string    `json:"request_no"`
	StudentID  uint64    `json:"student_id"`
	CourseID   uint64    `json:"course_id"`
	Action     string    `json:"action"`
	Status     string    `json:"status"`
	FailReason string    `json:"fail_reason"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func SelectionRequestStateKey(requestNo string) string {
	return fmt.Sprintf("selection:request:%s", requestNo)
}

// StorePendingSelectionRequestState records a request as accepted in Redis and
// indexes it by deadline for timeout sweeping.
func StorePendingSelectionRequestState(ctx context.Context, state SelectionRequestState, timeout, ttl time.Duration) error {
	client, err := redisClient()
	if err != nil {
		return err
	}

	now := time.Now()
	if state.CreatedAt.IsZero() {
		state.CreatedAt = now
	}
	state.UpdatedAt = now

	body, err := json.Marshal(state)
	if err != nil {
		return err
	}

	deadline := now.Add(timeout)
	pipe := client.Pipeline()
	pipe.Set(ctx, SelectionRequestStateKey(state.RequestNo), body, ttl)
	pipe.ZAdd(ctx, selectionRequestPendingDeadlineKey, redis.Z{
		Score:  float64(deadline.Unix()),
		Member: state.RequestNo,
	})
	_, err = pipe.Exec(ctx)
	return err
}

// StoreFinalSelectionRequestState keeps a brief Redis copy of the final state so
// immediate polls do not see stale pending data while MySQL is already final.
func StoreFinalSelectionRequestState(ctx context.Context, state SelectionRequestState, ttl time.Duration) error {
	client, err := redisClient()
	if err != nil {
		return err
	}

	state.UpdatedAt = time.Now()
	body, err := json.Marshal(state)
	if err != nil {
		return err
	}

	pipe := client.Pipeline()
	pipe.Set(ctx, SelectionRequestStateKey(state.RequestNo), body, ttl)
	pipe.ZRem(ctx, selectionRequestPendingDeadlineKey, state.RequestNo)
	_, err = pipe.Exec(ctx)
	return err
}

// GetSelectionRequestState returns nil when the Redis request state does not
// exist. The caller can then fall back to MySQL.
func GetSelectionRequestState(ctx context.Context, requestNo string) (*SelectionRequestState, error) {
	client, err := redisClient()
	if err != nil {
		return nil, err
	}

	body, err := client.Get(ctx, SelectionRequestStateKey(requestNo)).Bytes()
	if err != nil {
		if isRedisNil(err) {
			return nil, nil
		}
		return nil, err
	}

	var state SelectionRequestState
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// DeleteSelectionRequestState removes both the request state and its pending
// timeout index entry.
func DeleteSelectionRequestState(ctx context.Context, requestNo string) error {
	client, err := redisClient()
	if err != nil {
		return err
	}

	pipe := client.Pipeline()
	pipe.Del(ctx, SelectionRequestStateKey(requestNo))
	pipe.ZRem(ctx, selectionRequestPendingDeadlineKey, requestNo)
	_, err = pipe.Exec(ctx)
	return err
}

// DuePendingSelectionRequestStates returns Redis pending requests whose deadline
// has passed. Missing state keys are removed from the index as stale entries.
func DuePendingSelectionRequestStates(ctx context.Context, now time.Time, limit int64) ([]SelectionRequestState, error) {
	client, err := redisClient()
	if err != nil {
		return nil, err
	}

	requestNos, err := client.ZRangeByScore(ctx, selectionRequestPendingDeadlineKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    fmt.Sprintf("%d", now.Unix()),
		Offset: 0,
		Count:  limit,
	}).Result()
	if err != nil {
		return nil, err
	}

	states := make([]SelectionRequestState, 0, len(requestNos))
	for _, requestNo := range requestNos {
		state, err := GetSelectionRequestState(ctx, requestNo)
		if err != nil {
			return nil, err
		}
		if state == nil {
			_ = client.ZRem(ctx, selectionRequestPendingDeadlineKey, requestNo).Err()
			continue
		}

		states = append(states, *state)
	}

	return states, nil
}
