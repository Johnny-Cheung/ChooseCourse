-- 这个文件是“初始化测试数据脚本”。
-- 它的作用不是建表，而是给已经建好的表插入一批演示数据，
-- 方便你本地开发、联调和测试。
-- 当前项目的程序运行时不会自动执行这个文件。
-- 现在真正生效的测试数据初始化逻辑在：
-- internal/repository/migrate.go -> SeedInitialData()

-- 默认密码均为 123456。
-- 数据库里不能存明文密码，所以这里存的是 bcrypt 哈希：
-- $2a$10$VA86e6YC19PMwyiIcyF/jueBSoZTzpx6Vtvcv4ZSiBbbj4HgJrN46

-- 初始化默认管理员。
-- ON DUPLICATE KEY UPDATE 的意思是：
-- 如果 admin_no 已经存在，就不要再插一条新的。
INSERT INTO `admins` (`admin_no`, `password_hash`, `name`, `phone`, `status`)
VALUES ('A0001', '$2a$10$VA86e6YC19PMwyiIcyF/jueBSoZTzpx6Vtvcv4ZSiBbbj4HgJrN46', '系统管理员', '13800000001', 1)
ON DUPLICATE KEY UPDATE `admin_no` = `admin_no`;

-- 初始化测试学生。
INSERT INTO `students` (`student_no`, `password_hash`, `name`, `phone`, `credit_limit`, `credit_used`, `status`)
VALUES
('20230001', '$2a$10$VA86e6YC19PMwyiIcyF/jueBSoZTzpx6Vtvcv4ZSiBbbj4HgJrN46', '张三', '13800000011', 25, 0, 1),
('20230002', '$2a$10$VA86e6YC19PMwyiIcyF/jueBSoZTzpx6Vtvcv4ZSiBbbj4HgJrN46', '李四', '13800000012', 25, 0, 1),
('20230003', '$2a$10$VA86e6YC19PMwyiIcyF/jueBSoZTzpx6Vtvcv4ZSiBbbj4HgJrN46', '王五', '13800000013', 25, 0, 1)
ON DUPLICATE KEY UPDATE `student_no` = `student_no`;

-- 下面开始初始化测试课程。
-- 这里没有使用 ON DUPLICATE KEY，是因为课程表目前没有给
-- “课程名 + 教师名 + 时间片”设置联合唯一索引。
--
-- 所以这里改用：
-- INSERT ... SELECT ... WHERE NOT EXISTS ...
-- 含义是：先检查数据库里有没有相同课程，如果没有才插入。

INSERT INTO `courses` (`course_name`, `teacher_name`, `capacity`, `selected_count`, `time_slot`, `credit`, `status`, `like_count`, `comment_count`, `version`)
SELECT '高等数学', '李老师', 80, 0, 1, 4, 1, 0, 0, 1
WHERE NOT EXISTS (
  SELECT 1 FROM `courses` WHERE `course_name` = '高等数学' AND `teacher_name` = '李老师' AND `time_slot` = 1
);

INSERT INTO `courses` (`course_name`, `teacher_name`, `capacity`, `selected_count`, `time_slot`, `credit`, `status`, `like_count`, `comment_count`, `version`)
SELECT '大学英语', '王老师', 60, 0, 2, 3, 1, 0, 0, 1
WHERE NOT EXISTS (
  SELECT 1 FROM `courses` WHERE `course_name` = '大学英语' AND `teacher_name` = '王老师' AND `time_slot` = 2
);

INSERT INTO `courses` (`course_name`, `teacher_name`, `capacity`, `selected_count`, `time_slot`, `credit`, `status`, `like_count`, `comment_count`, `version`)
SELECT '数据结构', '陈老师', 50, 0, 5, 4, 1, 0, 0, 1
WHERE NOT EXISTS (
  SELECT 1 FROM `courses` WHERE `course_name` = '数据结构' AND `teacher_name` = '陈老师' AND `time_slot` = 5
);

INSERT INTO `courses` (`course_name`, `teacher_name`, `capacity`, `selected_count`, `time_slot`, `credit`, `status`, `like_count`, `comment_count`, `version`)
SELECT '体育', '赵老师', 100, 0, 8, 2, 1, 0, 0, 1
WHERE NOT EXISTS (
  SELECT 1 FROM `courses` WHERE `course_name` = '体育' AND `teacher_name` = '赵老师' AND `time_slot` = 8
);
