package cache

import "fmt"

const (
	// selectedSetSentinel 是学生已选课程集合里的一个“占位成员”。
	//
	// 为什么需要这个占位值：
	// Redis 的 Set 如果一个成员都没有，键本身就会不存在。
	// 但我们又希望“学生虽然还没选课，缓存也算已经初始化过”。
	//
	// 所以这里约定：
	// - 每次初始化 student:selected:{studentId} 时，至少写入这个占位成员
	// - 后续判断某门课是否已选时，只查真正的 courseId，不会受这个占位值影响
	selectedSetSentinel = "__loaded__"
)

// CourseStockKey 返回“课程剩余库存”这个 Redis 键。
//
// 这里存的是“还能再选多少人”，不是总容量。
// 例如课程容量 50、已选 12，那么这里就存 38。
func CourseStockKey(courseID uint64) string {
	// 这里只负责统一拼接键名，不做任何数据库或 Redis 操作。
	// 把键名收口到一个地方的好处是：
	// 后面如果要改命名规则，只改这一处即可。
	return fmt.Sprintf("course:stock:%d", courseID)
}

// CourseStatusKey 返回“课程开课状态”这个 Redis 键。
func CourseStatusKey(courseID uint64) string {
	// 课程状态缓存键。
	return fmt.Sprintf("course:status:%d", courseID)
}

// CourseCreditKey 返回“课程学分”这个 Redis 键。
func CourseCreditKey(courseID uint64) string {
	// 课程学分缓存键。
	return fmt.Sprintf("course:credit:%d", courseID)
}

// CourseSlotKey 返回“课程时间片”这个 Redis 键。
func CourseSlotKey(courseID uint64) string {
	// 课程时间片缓存键。
	return fmt.Sprintf("course:slot:%d", courseID)
}

// StudentSelectedKey 返回“学生已选课程集合”这个 Redis 键。
func StudentSelectedKey(studentID uint64) string {
	// 学生已选课程集合缓存键。
	return fmt.Sprintf("student:selected:%d", studentID)
}

// StudentCreditUsedKey 返回“学生已用学分”这个 Redis 键。
func StudentCreditUsedKey(studentID uint64) string {
	// 学生已用学分缓存键。
	return fmt.Sprintf("student:credit_used:%d", studentID)
}

// StudentCreditLimitKey 返回“学生学分上限”这个 Redis 键。
//
// 文档重点强调了 credit_used，但项目里每个学生也可能有不同的 credit_limit，
// 所以为了让 Redis 能独立完成“学分是否足够”的判断，这里也把学分上限一起缓存。
func StudentCreditLimitKey(studentID uint64) string {
	// 学生学分上限缓存键。
	return fmt.Sprintf("student:credit_limit:%d", studentID)
}

// StudentSlotBitmapKey 返回“学生已占用时间片位图”这个 Redis 键。
func StudentSlotBitmapKey(studentID uint64) string {
	// 学生时间片位图缓存键。
	return fmt.Sprintf("student:slot_bitmap:%d", studentID)
}
