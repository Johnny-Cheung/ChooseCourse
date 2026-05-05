package cache

import (
	"context"
	"errors"
	"strconv"

	"choose-course-backend/internal/model"
	"choose-course-backend/internal/repository"
	"gorm.io/gorm"
)

// EnsureStudentSelectionCache 确保某个学生的 Redis 缓存已经存在。
//
// 如果当前学生相关缓存不完整，就会从 MySQL 回源补建。
func EnsureStudentSelectionCache(ctx context.Context, studentID uint64) error {
	if missing, err := hasStudentMissingCache(ctx, studentID); err != nil {
		return err
	} else if missing {
		return ErrStudentSelectionCacheNotFound
	}

	// 先检查学生缓存是否完整存在。
	ready, err := hasStudentSelectionCache(ctx, studentID)
	if err != nil {
		return err
	}

	// 如果关键键都在，说明学生缓存已经就绪。
	if ready {
		return nil
	}

	token, locked, err := acquireSelectionCacheRebuildLock(ctx, StudentRebuildLockKey(studentID))
	if err != nil {
		return err
	}
	if !locked {
		return waitForStudentSelectionCache(ctx, studentID)
	}
	defer releaseSelectionCacheRebuildLock(ctx, StudentRebuildLockKey(studentID), token)

	if missing, err := hasStudentMissingCache(ctx, studentID); err != nil {
		return err
	} else if missing {
		return ErrStudentSelectionCacheNotFound
	}
	ready, err = hasStudentSelectionCache(ctx, studentID)
	if err != nil {
		return err
	}
	if ready {
		return nil
	}

	// 否则从 MySQL 重建学生缓存。
	return RefreshStudentSelectionCache(ctx, studentID)
}

// RefreshStudentSelectionCache 强制从 MySQL 刷新某个学生的缓存。
//
// 它会一起写入：
// 1. 已选课程集合
// 2. 已用学分
// 3. 学分上限
// 4. 时间片位图
func RefreshStudentSelectionCache(ctx context.Context, studentID uint64) error {
	// 先拿 Redis 客户端。
	client, err := redisClient()
	if err != nil {
		return err
	}

	// 先查学生本人的学分数据。
	// M6 里 Redis 要能独立做“学分是否足够”的判断，所以这里必须带上 credit_used 和 credit_limit。
	var student model.Student
	if err := repository.DB().
		Select("id", "credit_used", "credit_limit").
		First(&student, studentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = InvalidateStudentSelectionCache(ctx, studentID)
			_ = setStudentMissingCache(ctx, studentID)
			return ErrStudentSelectionCacheNotFound
		}
		return err
	}

	// 这里查询学生当前所有“仍然有效(status=1)”的已选课程，
	// 用来补建 selected 集合和时间片位图。
	var selectedCourses []struct {
		CourseID uint64 `gorm:"column:course_id"`
		TimeSlot int8   `gorm:"column:time_slot"`
	}
	if err := repository.DB().
		Table("enrollments").
		Select("enrollments.course_id, courses.time_slot").
		Joins("JOIN courses ON courses.id = enrollments.course_id").
		Where("enrollments.student_id = ? AND enrollments.status = 1", studentID).
		Scan(&selectedCourses).Error; err != nil {
		return err
	}

	// 构建课程集合成员。
	// 无论学生当前有没有选课，都至少写入一个 selectedSetSentinel，
	// 这样 student:selected:{studentId} 这个 Redis Set 键一定存在。
	members := make([]any, 0, len(selectedCourses)+1)
	members = append(members, selectedSetSentinel)

	// slotBitmap 是学生当前已占用时间片的位图表示。
	// 后面 Lua 判断冲突时，只需要做位运算，不用再查数据库。
	var slotBitmap uint64
	for _, item := range selectedCourses {
		// 课程集合里真正存的是 courseId 的字符串形式。
		members = append(members, strconv.FormatUint(item.CourseID, 10))
		// 把这门课的时间片映射进位图。
		slotBitmap |= timeSlotBitmap(item.TimeSlot)
	}

	selectedKey := StudentSelectedKey(studentID)

	// 这里也用 pipeline，一次性写入多个键。
	pipe := client.Pipeline()

	// 先删再写，避免旧集合残留脏成员。
	pipe.Del(ctx, selectedKey)
	// 写入“已选课程集合”。
	pipe.SAdd(ctx, selectedKey, members...)
	// 写入“已用学分”。
	pipe.Set(ctx, StudentCreditUsedKey(studentID), student.CreditUsed, 0)
	// 写入“学分上限”。
	pipe.Set(ctx, StudentCreditLimitKey(studentID), student.CreditLimit, 0)
	// 写入“时间片位图”。
	pipe.Set(ctx, StudentSlotBitmapKey(studentID), slotBitmap, 0)
	pipe.Del(ctx, StudentMissingKey(studentID))

	// 统一提交 pipeline。
	_, err = pipe.Exec(ctx)
	return err
}

// InvalidateStudentSelectionCache 删除学生相关缓存。
func InvalidateStudentSelectionCache(ctx context.Context, studentID uint64) error {
	// 一次性删掉这个学生与抢课前置校验相关的所有缓存键。
	client, err := redisClient()
	if err != nil {
		return err
	}

	return client.Del(
		ctx,
		StudentSelectedKey(studentID),
		StudentCreditUsedKey(studentID),
		StudentCreditLimitKey(studentID),
		StudentSlotBitmapKey(studentID),
		StudentMissingKey(studentID),
	).Err()
}

// hasStudentSelectionCache 判断学生缓存是否完整。
func hasStudentSelectionCache(ctx context.Context, studentID uint64) (bool, error) {
	// 学生缓存要求 4 个关键键都在，才算完整。
	client, err := redisClient()
	if err != nil {
		return false, err
	}

	exists, err := client.Exists(
		ctx,
		StudentSelectedKey(studentID),
		StudentCreditUsedKey(studentID),
		StudentCreditLimitKey(studentID),
		StudentSlotBitmapKey(studentID),
	).Result()
	if err != nil {
		return false, err
	}

	return exists == 4, nil
}

func hasStudentMissingCache(ctx context.Context, studentID uint64) (bool, error) {
	client, err := redisClient()
	if err != nil {
		return false, err
	}

	exists, err := client.Exists(ctx, StudentMissingKey(studentID)).Result()
	if err != nil {
		return false, err
	}

	return exists == 1, nil
}

func setStudentMissingCache(ctx context.Context, studentID uint64) error {
	client, err := redisClient()
	if err != nil {
		return err
	}

	return client.Set(ctx, StudentMissingKey(studentID), 1, selectionMissingCacheTTL()).Err()
}

func waitForStudentSelectionCache(ctx context.Context, studentID uint64) error {
	return waitForSelectionCacheReady(
		ctx,
		func(ctx context.Context) (bool, error) {
			return hasStudentSelectionCache(ctx, studentID)
		},
		func(ctx context.Context) (bool, error) {
			return hasStudentMissingCache(ctx, studentID)
		},
		ErrStudentSelectionCacheNotFound,
	)
}

// timeSlotBitmap 把单个 time_slot 映射成一个位图值。
//
// 例如：
// - time_slot = 1  -> 000...0001
// - time_slot = 2  -> 000...0010
// - time_slot = 5  -> 0001_0000
//
// 这样后面做时间冲突判断时，就可以直接做按位与运算。
func timeSlotBitmap(timeSlot int8) uint64 {
	// time_slot <= 0 表示“无有效时间片”，直接返回 0。
	if timeSlot <= 0 {
		return 0
	}

	// 例如：
	// time_slot = 1 -> 第 0 位是 1
	// time_slot = 2 -> 第 1 位是 1
	// 这样后面可以通过按位与判断是否冲突。
	return uint64(1) << uint(timeSlot-1)
}
