package cache

import (
	"context"
	"fmt"
	"strconv"

	"choose-course-backend/internal/repository"
	"github.com/redis/go-redis/v9"
)

// SelectPrecheckCode 表示 Redis Lua 预校验的结果码。
//
// 这里不用 error 直接表达“业务不通过”，是为了把两类问题区分开：
// 1. Redis 网络/执行异常 -> 真正的 error
// 2. 业务规则不通过 -> Lua 返回结果码
type SelectPrecheckCode int64

const (
	// SelectPrecheckOK 表示 Lua 预校验通过，并且已经完成了预扣减。
	SelectPrecheckOK SelectPrecheckCode = 0

	// 下面这些值分别对应抢课失败的不同业务原因。
	SelectPrecheckCourseClosed       SelectPrecheckCode = 1
	SelectPrecheckCourseFull         SelectPrecheckCode = 2
	SelectPrecheckAlreadySelected    SelectPrecheckCode = 3
	SelectPrecheckTimeConflict       SelectPrecheckCode = 4
	SelectPrecheckInsufficientCredit SelectPrecheckCode = 5

	// SelectPrecheckCacheMiss 表示 Lua 执行时发现关键缓存缺失。
	// 正常情况下我们在脚本前已经做了 Ensure，不应走到这里。
	SelectPrecheckCacheMiss SelectPrecheckCode = 99
)

// reserveSelectionScript 是 M6 抢课预校验的 Lua 脚本。
//
// 它会一次性完成这些事情：
// 1. 校验课程是否开课
// 2. 校验课程是否还有库存
// 3. 校验学生是否已选过
// 4. 校验时间冲突
// 5. 校验学分是否足够
// 6. 如果全部通过，就原子更新 Redis 里的库存、已选集合、时间位图、已用学分
var reserveSelectionScript = redis.NewScript(`

-- KEYS 说明：
-- 1 course:stock
-- 2 course:status
-- 3 course:credit
-- 4 course:slot
-- 5 student:selected
-- 6 student:credit_used
-- 7 student:slot_bitmap
-- 8 student:credit_limit
--
-- ARGV[1] 是当前课程的 courseId 字符串，用来往 student:selected 这个集合里写成员。

-- 先把课程和学生相关缓存读出来。
local stock = tonumber(redis.call("GET", KEYS[1]) or "-1")
local status = tonumber(redis.call("GET", KEYS[2]) or "-1")
local courseCredit = tonumber(redis.call("GET", KEYS[3]) or "-1")
local courseSlot = tonumber(redis.call("GET", KEYS[4]) or "0")
local creditUsed = tonumber(redis.call("GET", KEYS[6]) or "0")
local studentBitmap = tonumber(redis.call("GET", KEYS[7]) or "0")
local creditLimit = tonumber(redis.call("GET", KEYS[8]) or "-1")

-- 只要关键课程缓存缺失，就直接返回 99。
-- 正常情况下，Go 代码在执行 Lua 前已经做过 Ensure；
-- 如果这里还缺，说明缓存状态已经不完整。
if stock < 0 or status < 0 or courseCredit < 0 or creditLimit < 0 then
	return 99
end

-- 课程不是开课状态，直接失败。
if status ~= 1 then
	return 1
end

-- 课程库存为 0 或负数，说明没名额了。
if stock <= 0 then
	return 2
end

-- 如果学生的已选课程集合里已经有这门课，说明重复选课。
if redis.call("SISMEMBER", KEYS[5], ARGV[1]) == 1 then
	return 3
end

local updatedBitmap = studentBitmap
if courseSlot > 0 then
	-- 先把当前课程的 time_slot 转成位图。
	local courseBitmap = bit.lshift(1, courseSlot - 1)
	-- 如果学生已有位图和课程位图按位与不为 0，说明时间冲突。
	if bit.band(studentBitmap, courseBitmap) ~= 0 then
		return 4
	end
	-- 无冲突时，把当前课程的位信息合并进学生位图。
	updatedBitmap = bit.bor(studentBitmap, courseBitmap)
end

-- 学分不足则失败。
if creditUsed + courseCredit > creditLimit then
	return 5
end

-- 只有上面所有校验都通过，才真正开始更新 Redis。
-- 这几步会在 Lua 里原子执行。
-- 1. 库存减 1
redis.call("DECR", KEYS[1])
-- 2. 把这门课写进学生已选集合
redis.call("SADD", KEYS[5], ARGV[1])
-- 3. 更新学生已用学分
redis.call("SET", KEYS[6], creditUsed + courseCredit)
-- 4. 更新学生时间位图
redis.call("SET", KEYS[7], updatedBitmap)

return 0

`)

// releaseReservedSelectionScript 用来“回退一次 Redis 预扣减”。
//
// 这里特别强调一个安全原则：
// 如果脚本发现关键缓存缺失，或者根本看不到“这次预扣减留下的痕迹”，
// 就直接返回错误，而不是用 0 当默认值继续写回 Redis。
//
// 原因是：
// 补偿场景下最怕的不是“补偿失败”，而是“补偿假装成功，但把缓存写脏了”。
// 一旦脚本返回错误，Go 代码就会进入更保守的兜底路径：删除相关缓存，让下一次请求回源 MySQL 重建。
var releaseReservedSelectionScript = redis.NewScript(`
-- 补偿脚本执行前，先确认关键缓存都存在。
if redis.call("EXISTS", KEYS[1], KEYS[2], KEYS[3], KEYS[4], KEYS[5], KEYS[6], KEYS[7], KEYS[8]) < 8 then
	return redis.error_reply("selection cache missing")
end

-- 如果学生已选集合里根本没有这门课，说明“这次预扣减的痕迹”都不存在，
-- 这时继续补偿只会把缓存写乱，所以直接报错。
if redis.call("SISMEMBER", KEYS[5], ARGV[1]) ~= 1 then
	return redis.error_reply("selection reservation missing")
end

-- 读取补偿需要的缓存字段。
local stock = tonumber(redis.call("GET", KEYS[1]) or "0")
local courseCredit = tonumber(redis.call("GET", KEYS[3]) or "0")
local courseSlot = tonumber(redis.call("GET", KEYS[4]) or "0")
local creditUsed = tonumber(redis.call("GET", KEYS[6]) or "0")
local studentBitmap = tonumber(redis.call("GET", KEYS[7]) or "0")

-- 把库存加回去。
redis.call("SET", KEYS[1], stock + 1)
-- 把课程从学生已选集合里删掉。
redis.call("SREM", KEYS[5], ARGV[1])

if courseSlot > 0 then
	local courseBitmap = bit.lshift(1, courseSlot - 1)
	-- 从学生时间位图里去掉这门课占用的那个 bit。
	local updatedBitmap = bit.band(studentBitmap, bit.bnot(courseBitmap))
	if updatedBitmap < 0 then
		updatedBitmap = 0
	end
	redis.call("SET", KEYS[7], updatedBitmap)
end

-- 学分也要回退。
local updatedCredit = creditUsed - courseCredit
if updatedCredit < 0 then
	updatedCredit = 0
end
redis.call("SET", KEYS[6], updatedCredit)

return 0
`)

// PrecheckAndReserveSelection 先确保缓存存在，再执行 Lua 原子预校验。
//
// 如果返回值是 SelectPrecheckOK，说明：
// - Redis 里的业务校验已经通过
// - Redis 里的库存/学分/已选状态也已经做了预更新
func PrecheckAndReserveSelection(ctx context.Context, studentID, courseID uint64) (SelectPrecheckCode, error) {
	// 在执行 Lua 之前，先确保课程和学生缓存都已经就绪。
	// 这是为了避免 Lua 因为关键缓存缺失而直接失效。
	if err := EnsureCourseSelectionCache(ctx, courseID); err != nil {
		return SelectPrecheckCacheMiss, err
	}
	if err := EnsureStudentSelectionCache(ctx, studentID); err != nil {
		return SelectPrecheckCacheMiss, err
	}

	// 拿到底层 Redis 客户端。
	client := repository.Redis()
	if client == nil {
		return SelectPrecheckCacheMiss, fmt.Errorf("redis not initialized")
	}

	// 执行 Lua 脚本。
	// 这里传入的是：
	// - 按固定顺序组装好的 KEYS
	// - courseId 作为 ARGV[1]
	result, err := reserveSelectionScript.Run(
		ctx,
		client,
		selectScriptKeys(studentID, courseID),
		strconv.FormatUint(courseID, 10),
	).Int64()
	if err != nil {
		return SelectPrecheckCacheMiss, err
	}

	// 把 Lua 返回的 int64 结果码转换成 Go 里的枚举类型。
	return SelectPrecheckCode(result), nil
}

// CompensateReservedSelection 回退一次已经成功执行过的 Redis 预扣减。
//
// 如果脚本返回错误，调用方不应继续相信当前缓存，
// 而应该走更保守的兜底逻辑，例如删除缓存，等待下一次请求回源 MySQL 重建。
func CompensateReservedSelection(ctx context.Context, studentID, courseID uint64) error {
	// 拿 Redis 客户端。
	client := repository.Redis()
	if client == nil {
		return fmt.Errorf("redis not initialized")
	}

	// 执行补偿脚本。
	// 一旦这里报错，上层就不应再信任当前缓存，而要走“删缓存 + 下次回源”的兜底路径。
	_, err := releaseReservedSelectionScript.Run(
		ctx,
		client,
		selectScriptKeys(studentID, courseID),
		strconv.FormatUint(courseID, 10),
	).Result()
	return err
}

// selectScriptKeys 把 Lua 脚本需要的键统一按顺序组装出来。
//
// 这样脚本本身就不用关心 Go 代码里具体怎么拼 key 了。
func selectScriptKeys(studentID, courseID uint64) []string {
	// 这里专门保证 Lua 用到的 KEYS 顺序固定。
	// 因为 Lua 脚本里是按 KEYS[1]、KEYS[2] ... 这样取值的，
	// 一旦顺序乱了，整个脚本逻辑都会错位。
	return []string{
		CourseStockKey(courseID),
		CourseStatusKey(courseID),
		CourseCreditKey(courseID),
		CourseSlotKey(courseID),
		StudentSelectedKey(studentID),
		StudentCreditUsedKey(studentID),
		StudentSlotBitmapKey(studentID),
		StudentCreditLimitKey(studentID),
	}
}
