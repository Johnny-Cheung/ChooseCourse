package cache

import (
	"context"
	"errors"
	"fmt"

	"choose-course-backend/internal/model"
	"choose-course-backend/internal/repository"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// EnsureCourseSelectionCache 确保某门课的 Redis 缓存已经存在。
//
// 它的行为是：
// - 如果缓存已经完整存在，就直接返回
// - 如果有任何一个键缺失，就从 MySQL 重新加载整门课的缓存
func EnsureCourseSelectionCache(ctx context.Context, courseID uint64) error {
	if missing, err := hasCourseMissingCache(ctx, courseID); err != nil {
		return err
	} else if missing {
		return ErrCourseSelectionCacheNotFound
	}

	// 先判断课程缓存是否已经完整存在。
	ready, err := hasCourseSelectionCache(ctx, courseID)
	if err != nil {
		return err
	}

	// 如果 4 个关键课程键都在，就说明缓存可直接用，不需要回源。
	if ready {
		return nil
	}

	token, locked, err := acquireSelectionCacheRebuildLock(ctx, CourseRebuildLockKey(courseID))
	if err != nil {
		return err
	}
	if !locked {
		return waitForCourseSelectionCache(ctx, courseID)
	}
	defer releaseSelectionCacheRebuildLock(ctx, CourseRebuildLockKey(courseID), token)

	if missing, err := hasCourseMissingCache(ctx, courseID); err != nil {
		return err
	} else if missing {
		return ErrCourseSelectionCacheNotFound
	}
	ready, err = hasCourseSelectionCache(ctx, courseID)
	if err != nil {
		return err
	}
	if ready {
		return nil
	}

	// 只要有任何一个键缺失，就统一从 MySQL 重建课程缓存。
	return RefreshCourseSelectionCache(ctx, courseID)
}

// RefreshCourseSelectionCache 强制从 MySQL 刷新一门课的缓存。
//
// 这一步会把课程相关的 4 个关键字段一起写进 Redis：
// 1. 剩余库存
// 2. 开课状态
// 3. 课程学分
// 4. 课程时间片
func RefreshCourseSelectionCache(ctx context.Context, courseID uint64) error {
	// 先拿 Redis 客户端。
	client, err := redisClient()
	if err != nil {
		return err
	}

	// 从 MySQL 读取构建课程缓存真正需要的字段。
	// 这里只查 capacity / selected_count / status / credit / time_slot，
	// 因为课程缓存目前只服务于抢课前置校验。
	var course model.Course
	if err := repository.DB().
		Select("id", "capacity", "selected_count", "status", "credit", "time_slot").
		First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = InvalidateCourseSelectionCache(ctx, courseID)
			_ = setCourseMissingCache(ctx, courseID)
			return ErrCourseSelectionCacheNotFound
		}
		return err
	}

	// Redis 里存的是“剩余库存”。
	// 如果数据库里 selected_count 已经大于 capacity（理论上不应该发生），
	// 这里也至少保证缓存里的库存不会变成负数。
	remainingStock := course.Capacity - course.SelectedCount
	if remainingStock < 0 {
		remainingStock = 0
	}

	// 用 pipeline 一次性写入 4 个键，减少网络往返次数。
	pipe := client.Pipeline()

	// 课程剩余库存。
	pipe.Set(ctx, CourseStockKey(courseID), remainingStock, 0)
	// 课程开课状态。
	pipe.Set(ctx, CourseStatusKey(courseID), course.Status, 0)
	// 课程学分。
	pipe.Set(ctx, CourseCreditKey(courseID), course.Credit, 0)
	// 课程时间片。
	pipe.Set(ctx, CourseSlotKey(courseID), course.TimeSlot, 0)
	pipe.Del(ctx, CourseMissingKey(courseID))

	// 统一执行 pipeline 里的写操作。
	_, err = pipe.Exec(ctx)
	return err
}

// InvalidateCourseSelectionCache 删除课程相关缓存。
//
// 这样做的目的不是“修好缓存”，而是“让坏缓存消失”，
// 这样下一次请求进来时就会自动回源 MySQL 重建。
func InvalidateCourseSelectionCache(ctx context.Context, courseID uint64) error {
	// 直接删除整门课相关的 4 个缓存键。
	// 这样后续只要再有人访问这门课，就会自动从 MySQL 回源重建。
	client, err := redisClient()
	if err != nil {
		return err
	}

	return client.Del(
		ctx,
		CourseStockKey(courseID),
		CourseStatusKey(courseID),
		CourseCreditKey(courseID),
		CourseSlotKey(courseID),
		CourseMissingKey(courseID),
	).Err()
}

// hasCourseSelectionCache 判断课程缓存是否“完整可用”。
//
// 这里要求 4 个键都存在，才算缓存准备完成。
func hasCourseSelectionCache(ctx context.Context, courseID uint64) (bool, error) {
	// 这里只关心“完整性”，不关心键里的值是否合理。
	// 如果存在数量是 4，说明课程缓存的最小闭环已就绪。
	client, err := redisClient()
	if err != nil {
		return false, err
	}

	exists, err := client.Exists(
		ctx,
		CourseStockKey(courseID),
		CourseStatusKey(courseID),
		CourseCreditKey(courseID),
		CourseSlotKey(courseID),
	).Result()
	if err != nil {
		return false, err
	}

	return exists == 4, nil
}

func hasCourseMissingCache(ctx context.Context, courseID uint64) (bool, error) {
	client, err := redisClient()
	if err != nil {
		return false, err
	}

	exists, err := client.Exists(ctx, CourseMissingKey(courseID)).Result()
	if err != nil {
		return false, err
	}

	return exists == 1, nil
}

func setCourseMissingCache(ctx context.Context, courseID uint64) error {
	client, err := redisClient()
	if err != nil {
		return err
	}

	return client.Set(ctx, CourseMissingKey(courseID), 1, selectionMissingCacheTTL()).Err()
}

func waitForCourseSelectionCache(ctx context.Context, courseID uint64) error {
	return waitForSelectionCacheReady(
		ctx,
		func(ctx context.Context) (bool, error) {
			return hasCourseSelectionCache(ctx, courseID)
		},
		func(ctx context.Context) (bool, error) {
			return hasCourseMissingCache(ctx, courseID)
		},
		ErrCourseSelectionCacheNotFound,
	)
}

// redisClient 是 cache 包内部统一取 Redis 客户端的小工具函数。
func redisClient() (*redis.Client, error) {
	// repository.Redis() 返回的是全局 Redis 客户端。
	// 如果服务还没初始化 Redis，这里直接报错，提醒上层不能继续执行缓存逻辑。
	client := repository.Redis()
	if client == nil {
		return nil, fmt.Errorf("redis not initialized")
	}

	return client, nil
}

func isRedisNil(err error) bool {
	return err == redis.Nil
}
