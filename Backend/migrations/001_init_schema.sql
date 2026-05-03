-- 这个文件是 M1 的“数据库结构脚本”。
-- 作用不是插入业务数据，而是把整个项目需要的表结构建出来。
-- 当前项目的程序运行时不会自动执行这个文件。
-- 现在真正生效的建表方式是：cmd/migrate -> repository.AutoMigrate()
--
-- 建议把它理解成“数据库骨架”：
-- 1. 先建表
-- 2. 再加索引
-- 3. 再加外键和约束

-- 学生表：
-- 存学生账号和基础资料。
CREATE TABLE IF NOT EXISTS `students` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `student_no` VARCHAR(32) NOT NULL COMMENT '学号',
  `password_hash` VARCHAR(255) NOT NULL COMMENT '密码摘要',
  `name` VARCHAR(64) NOT NULL COMMENT '姓名',
  `phone` VARCHAR(20) NOT NULL COMMENT '手机号',
  `credit_limit` INT NOT NULL DEFAULT 25 COMMENT '总学分上限',
  `credit_used` INT NOT NULL DEFAULT 0 COMMENT '已用学分',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态 1正常 0禁用',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` DATETIME NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_student_no` (`student_no`),
  KEY `idx_students_phone` (`phone`),
  KEY `idx_students_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='学生表';

-- 管理员表：
-- 存管理员账号和基础资料。
CREATE TABLE IF NOT EXISTS `admins` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `admin_no` VARCHAR(32) NOT NULL COMMENT '工号',
  `password_hash` VARCHAR(255) NOT NULL COMMENT '密码摘要',
  `name` VARCHAR(64) NOT NULL COMMENT '姓名',
  `phone` VARCHAR(20) NOT NULL COMMENT '手机号',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态 1正常 0禁用',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` DATETIME NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_admin_no` (`admin_no`),
  KEY `idx_admins_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='管理员表';

-- 课程表：
-- 存课程基础信息，以及冗余计数信息。
-- 说明：
-- like_count、comment_count、selected_count 都是冗余字段，
-- 目的是让课程列表查询更快。
CREATE TABLE IF NOT EXISTS `courses` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `course_name` VARCHAR(128) NOT NULL COMMENT '课程名称',
  `teacher_name` VARCHAR(64) NOT NULL COMMENT '教师姓名',
  `capacity` INT NOT NULL COMMENT '课程容量',
  `selected_count` INT NOT NULL DEFAULT 0 COMMENT '已选人数',
  `time_slot` TINYINT NOT NULL COMMENT '时间片 0-20',
  `credit` TINYINT NOT NULL COMMENT '学分 2/3/4',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态 0不开课 1开课',
  `like_count` INT NOT NULL DEFAULT 0 COMMENT '点赞数',
  `comment_count` INT NOT NULL DEFAULT 0 COMMENT '评论数',
  `version` INT NOT NULL DEFAULT 1 COMMENT '乐观锁版本号',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` DATETIME NULL DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (`id`),
  KEY `idx_courses_course_name` (`course_name`),
  KEY `idx_courses_teacher_name` (`teacher_name`),
  KEY `idx_courses_status_time_slot` (`status`, `time_slot`),
  KEY `idx_courses_deleted_at` (`deleted_at`),
  CONSTRAINT `chk_courses_capacity` CHECK (`capacity` > 0),
  CONSTRAINT `chk_courses_selected_count` CHECK (`selected_count` >= 0),
  CONSTRAINT `chk_courses_time_slot` CHECK (`time_slot` BETWEEN 0 AND 20),
  CONSTRAINT `chk_courses_credit` CHECK (`credit` IN (2, 3, 4)),
  CONSTRAINT `chk_courses_status` CHECK (`status` IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='课程表';

-- 选课记录表：
-- 负责把“学生”和“课程”关联起来。
-- 一条记录就表示某个学生和某门课之间存在一条选课关系。
CREATE TABLE IF NOT EXISTS `enrollments` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `student_id` BIGINT UNSIGNED NOT NULL COMMENT '学生ID',
  `course_id` BIGINT UNSIGNED NOT NULL COMMENT '课程ID',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态 1已选 0已退',
  `selected_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '选课时间',
  `dropped_at` DATETIME NULL DEFAULT NULL COMMENT '退课时间',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_student_course` (`student_id`, `course_id`),
  KEY `idx_enrollments_student_status` (`student_id`, `status`),
  KEY `idx_enrollments_course_status` (`course_id`, `status`),
  CONSTRAINT `fk_enrollments_student` FOREIGN KEY (`student_id`) REFERENCES `students` (`id`),
  CONSTRAINT `fk_enrollments_course` FOREIGN KEY (`course_id`) REFERENCES `courses` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='选课记录表';

-- 课程点赞表：
-- 通过 student_id + course_id 联合唯一索引，保证同一学生对同一课程只能点赞一次。
CREATE TABLE IF NOT EXISTS `course_likes` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `student_id` BIGINT UNSIGNED NOT NULL COMMENT '学生ID',
  `course_id` BIGINT UNSIGNED NOT NULL COMMENT '课程ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_like_student_course` (`student_id`, `course_id`),
  KEY `idx_course_likes_course_id` (`course_id`),
  CONSTRAINT `fk_course_likes_student` FOREIGN KEY (`student_id`) REFERENCES `students` (`id`),
  CONSTRAINT `fk_course_likes_course` FOREIGN KEY (`course_id`) REFERENCES `courses` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='课程点赞表';

-- 课程评论表：
-- status 字段用来表示评论是否仍然可见。
CREATE TABLE IF NOT EXISTS `course_comments` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `student_id` BIGINT UNSIGNED NOT NULL COMMENT '学生ID',
  `course_id` BIGINT UNSIGNED NOT NULL COMMENT '课程ID',
  `content` VARCHAR(500) NOT NULL COMMENT '评论内容',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态 1正常 0已删除',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_course_comments_course_created` (`course_id`, `created_at`),
  KEY `idx_course_comments_student_created` (`student_id`, `created_at`),
  CONSTRAINT `fk_course_comments_student` FOREIGN KEY (`student_id`) REFERENCES `students` (`id`),
  CONSTRAINT `fk_course_comments_course` FOREIGN KEY (`course_id`) REFERENCES `courses` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='课程评论表';

-- 消息通知表：
-- 用于存放系统消息，例如抢课成功、退课成功、评论成功等。
CREATE TABLE IF NOT EXISTS `notifications` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `biz_key` VARCHAR(128) NOT NULL COMMENT '通知幂等键',
  `recipient_type` TINYINT NOT NULL COMMENT '接收人类型 1学生 2管理员',
  `recipient_id` BIGINT UNSIGNED NOT NULL COMMENT '接收人ID',
  `biz_type` VARCHAR(32) NOT NULL COMMENT '业务类型',
  `title` VARCHAR(128) NOT NULL COMMENT '标题',
  `content` VARCHAR(500) NOT NULL COMMENT '内容',
  `related_course_id` BIGINT UNSIGNED NULL DEFAULT NULL COMMENT '关联课程ID',
  `related_comment_id` BIGINT UNSIGNED NULL DEFAULT NULL COMMENT '关联评论ID',
  `is_read` TINYINT NOT NULL DEFAULT 0 COMMENT '是否已读',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notifications_biz_key` (`biz_key`),
  KEY `idx_notifications_recipient_read_created` (`recipient_type`, `recipient_id`, `is_read`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='消息通知表';

-- 抢课请求表：
-- 用于记录一次抢课/退课请求本身的处理状态。
-- 这是后续高并发异步化的重要基础表。
CREATE TABLE IF NOT EXISTS `selection_requests` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `request_no` VARCHAR(64) NOT NULL COMMENT '请求号',
  `student_id` BIGINT UNSIGNED NOT NULL COMMENT '学生ID',
  `course_id` BIGINT UNSIGNED NOT NULL COMMENT '课程ID',
  `action` VARCHAR(16) NOT NULL COMMENT '动作 grab/drop',
  `status` VARCHAR(16) NOT NULL COMMENT '状态 pending/success/failed',
  `fail_reason` VARCHAR(255) NULL DEFAULT NULL COMMENT '失败原因',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_no` (`request_no`),
  KEY `idx_selection_requests_student_created` (`student_id`, `created_at`),
  KEY `idx_selection_requests_course_created` (`course_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='抢课请求表';
