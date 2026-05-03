# 高校高并发抢课系统后端功能技术需求文档

- 文档版本：v0.1
- 编写日期：2026-03-31
- 适用范围：`ChooseCourse` 后端项目一期
- 目标技术栈：Go + Gin + Gorm + MySQL + Redis + RabbitMQ + Viper

## 1. 项目背景与目标

本项目用于实现一个高校在线抢课系统后端，支持学生和管理员两类用户。系统需要同时满足常规教务管理能力与高并发抢课能力，核心目标如下：

1. 支持学生登录、查课、点赞、评论、抢课、退课、查看课表和消息通知。
2. 支持管理员登录、学生管理、课程管理和按学号查询学生信息。
3. 在高并发抢课场景下，保证课程不超选、不可重复选课、无课程时间冲突、学生学分额度不超限。
4. 通过 Redis 和 RabbitMQ 提升系统吞吐能力，并通过 MySQL 保证最终一致性。

## 2. 用户角色与权限

### 2.1 学生

学生信息字段：

- 学号
- 密码
- 姓名
- 手机号
- 可选学分（默认 25）

学生可执行功能：

1. 登录系统
2. 查看个人信息
3. 修改个人信息
4. 查看所有课程
5. 搜索课程
6. 给课程点赞
7. 取消点赞
8. 给课程评论
9. 删除自己的评论
10. 抢课
11. 退课
12. 查看课程表
13. 查看消息通知

学生个人信息修改限制：

- 姓名不可修改
- 学号不可修改
- 可修改字段：密码、手机号

### 2.2 管理员

管理员信息字段：

- 工号
- 密码
- 姓名
- 手机号

管理员可执行功能：

1. 登录系统
2. 查看个人信息
3. 修改个人信息
4. 添加学生
5. 新增课程
6. 修改课程信息
7. 修改学生信息
8. 通过学号查询学生信息

管理员修改限制建议：

- 工号不可修改
- 姓名建议不可修改，若允许修改则走个人信息修改接口
- 不建议在常规业务接口中修改学生学号和管理员工号

## 3. 一期功能范围

### 3.1 认证与授权

1. 学生登录
2. 管理员登录
3. 基于 JWT 的身份认证
4. 基于角色的接口访问控制

### 3.2 学生端功能

1. 查看个人资料
2. 修改个人资料
3. 查看课程列表
4. 课程条件搜索
5. 课程点赞与取消点赞
6. 课程评论与删除自己的评论
7. 抢课
8. 退课
9. 查看个人课表
10. 查看消息列表
11. 标记消息已读

### 3.3 管理端功能

1. 新增学生
2. 修改学生信息
3. 根据学号查询学生信息
4. 新增课程
5. 修改课程信息
6. 查看个人资料
7. 修改个人资料

### 3.4 核心并发功能

1. 高并发抢课请求入口
2. Redis 预扣减课程容量
3. Redis 快速判重
4. Redis 快速校验时间冲突与学分限制
5. RabbitMQ 异步削峰
6. MySQL 事务落库
7. 失败补偿与消息通知

## 4. 核心业务规则

### 4.1 学生信息规则

1. 学号唯一。
2. 手机号建议唯一。
3. 密码仅存储加密摘要，不存明文。
4. 学生默认可选总学分为 25。

### 4.2 课程规则

课程基础字段：

- 课程名称
- 上课老师
- 课程容量
- 已选人数
- 上课时间
- 学分
- 开课状态

课程业务约束：

1. 课程容量必须大于 0。
2. 已选人数不能大于课程容量。
3. 学分只允许为 2、3、4。
4. 开课状态只有 `0-不开课`、`1-开课`。
5. 只有开课状态为 `1` 的课程可被学生选择。
6. 课程有学生已选择后，不可改为不开课。
7. 课程有学生已选择后，不建议修改课程时间和学分。
8. 课程容量可调大，但若调小则不得小于已选人数。

### 4.3 上课时间规则

按题意，一期采用单时间片模型：

- `1-20` 分别表示周一到周五的上午第一节、第二节、下午第一节、第二节
- `0` 表示不开课或未排课

说明：

1. 一期数据库采用单字段 `time_slot` 存储即可。
2. 若后续需扩展为一门课多个时间片，可拆分为 `course_schedules` 子表。

### 4.4 抢课核心规则

学生抢课时必须同时满足以下条件：

1. 课程开课状态为开课。
2. 课程容量有剩余，不允许超选。
3. 学生未选过该课程。
4. 课程时间与学生已选课程无冲突。
5. 学生剩余可选学分足够。

### 4.5 退课规则

1. 学生只能退自己已选的课程。
2. 退课成功后，已选人数减 1。
3. 退课成功后，学生已用学分减少。
4. 退课成功后，向学生发送成功消息。

### 4.6 点赞与评论规则

1. 同一学生对同一课程只能点赞一次。
2. 取消点赞后可再次点赞。
3. 学生只能删除自己发表的评论。
4. 评论内容不能为空，建议长度限制为 1-500 字。
5. 点赞、取消点赞、评论、删除评论成功后，应写入消息通知。

## 5. 推荐技术栈

### 5.1 必选技术栈

- Web 框架：Gin
- ORM：Gorm
- 数据库：MySQL 8.x
- 缓存：Redis
- 消息队列：RabbitMQ
- 配置管理：Viper

### 5.2 推荐新增依赖

- JWT：`github.com/golang-jwt/jwt/v5`
- 参数校验：`github.com/go-playground/validator/v10`
- 密码加密：`golang.org/x/crypto/bcrypt`
- 日志：`go.uber.org/zap`
- 接口文档：`github.com/swaggo/swag`
- 唯一 ID：雪花算法或 UUID
- 链路追踪与监控：Prometheus + Grafana
- 限流：Redis + 中间件

## 6. 后端架构设计

建议采用分层架构：

1. `handler`：处理 HTTP 请求、参数绑定、权限校验
2. `service`：处理业务逻辑
3. `repository/dao`：数据库与缓存访问
4. `model`：实体与 DTO
5. `mq`：RabbitMQ 生产和消费
6. `pkg`：JWT、响应封装、错误码、日志、工具函数
7. `config`：Viper 配置加载

推荐目录结构：

```text
Backend/
├─ cmd/
│  └─ server/
├─ configs/
├─ docs/
├─ internal/
│  ├─ handler/
│  ├─ service/
│  ├─ repository/
│  ├─ model/
│  ├─ middleware/
│  ├─ mq/
│  ├─ cache/
│  └─ pkg/
├─ migrations/
├─ scripts/
└─ go.mod
```

## 7. 抢课高并发方案设计

## 7.1 设计目标

1. 防止课程超卖
2. 防止重复抢课
3. 快速判断时间冲突
4. 快速判断学分是否足够
5. 在高并发下保护 MySQL
6. 保证最终一致性

### 7.2 推荐处理流程

1. 学生发起抢课请求。
2. 网关或 Gin 中间件完成认证、限流和参数校验。
3. 服务层调用 Redis Lua 脚本进行原子预检查：
   - 检查课程开课状态
   - 检查剩余库存
   - 检查是否已选
   - 检查时间冲突
   - 检查剩余学分
4. Lua 校验通过后：
   - 扣减 Redis 课程库存
   - 写入学生已选课程缓存
   - 更新学生时间片位图缓存
   - 更新学生已用学分缓存
   - 生成抢课请求记录
5. 将抢课事件投递到 RabbitMQ。
6. 消费者串行或分片消费，执行 MySQL 事务：
   - 再次校验课程状态和库存
   - 插入选课记录
   - 更新课程已选人数
   - 更新学生已用学分
   - 写入消息通知
   - 更新请求状态为成功
7. 若数据库事务失败：
   - 回滚事务
   - 执行 Redis 补偿
   - 将请求状态标记为失败
   - 发送失败消息或写入失败原因

### 7.3 Redis 设计建议

推荐缓存键：

- `course:stock:{courseId}`：课程剩余名额
- `course:status:{courseId}`：课程开课状态
- `course:credit:{courseId}`：课程学分
- `course:slot:{courseId}`：课程时间片
- `student:selected:{studentId}`：学生已选课程集合
- `student:credit_used:{studentId}`：学生已用学分
- `student:slot_bitmap:{studentId}`：学生已选课程时间位图
- `course:like_count:{courseId}`：课程点赞数
- `course:comment_count:{courseId}`：课程评论数

时间冲突推荐实现：

1. 将 `1-20` 时间片映射为 20 位二进制位图。
2. 课程时间片转换为一个 bit。
3. 学生已选课表缓存为位图整数。
4. 抢课时执行位运算：
   - `(studentBitmap & courseBitmap) != 0` 则表示冲突

### 7.4 RabbitMQ 设计建议

推荐交换机与队列：

- 交换机：`course.selection.exchange`
- 队列：`course.selection.grab.queue`
- 队列：`course.selection.drop.queue`
- 队列：`notification.queue`
- 死信队列：`course.selection.dlx.queue`

建议：

1. 队列持久化
2. 消息持久化
3. 手动 ACK
4. 失败重试 + 死信队列
5. 通过课程 ID 分片路由，降低同一课程并发写冲突

### 7.5 数据一致性策略

1. Redis 负责高并发前置拦截。
2. MySQL 负责最终权威数据。
3. 抢课成功以 MySQL 事务提交结果为准。
4. Redis 与 MySQL 不一致时，以补偿任务或定时任务修正。
5. 课程已选人数 `selected_count` 为冗余字段，需与选课表保持一致。

### 7.6 幂等与去重

1. 抢课请求需携带 `request_id` 或 `idempotency_key`。
2. 选课表增加唯一索引 `(student_id, course_id)`。
3. 点赞表增加唯一索引 `(student_id, course_id)`。
4. 抢课请求表增加唯一索引 `request_no`。
5. MQ 消费端必须做幂等校验，避免重复消费。

## 8. 数据库表设计

以下为一期核心表设计建议。

### 8.1 学生表 `students`

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| student_no | varchar(32) | 学号，唯一 |
| password_hash | varchar(255) | 密码摘要 |
| name | varchar(64) | 姓名 |
| phone | varchar(20) | 手机号 |
| credit_limit | int | 总学分上限，默认 25 |
| credit_used | int | 已使用学分，默认 0 |
| status | tinyint | 状态，1 正常，0 禁用 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |
| deleted_at | datetime null | 软删除时间 |

索引建议：

- 唯一索引：`uk_student_no`
- 普通索引：`idx_phone`

### 8.2 管理员表 `admins`

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| admin_no | varchar(32) | 工号，唯一 |
| password_hash | varchar(255) | 密码摘要 |
| name | varchar(64) | 姓名 |
| phone | varchar(20) | 手机号 |
| status | tinyint | 状态，1 正常，0 禁用 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |
| deleted_at | datetime null | 软删除时间 |

索引建议：

- 唯一索引：`uk_admin_no`

### 8.3 课程表 `courses`

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| course_name | varchar(128) | 课程名称 |
| teacher_name | varchar(64) | 上课教师 |
| capacity | int | 课程容量 |
| selected_count | int | 已选人数 |
| time_slot | tinyint | 上课时间片，0-20 |
| credit | tinyint | 学分，只允许 2/3/4 |
| status | tinyint | 开课状态，0 不开课，1 开课 |
| like_count | int | 点赞数，冗余字段 |
| comment_count | int | 评论数，冗余字段 |
| version | int | 乐观锁版本号，可选 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |
| deleted_at | datetime null | 软删除时间 |

索引建议：

- 普通索引：`idx_course_name`
- 普通索引：`idx_teacher_name`
- 联合索引：`idx_status_time_slot`

### 8.4 选课表 `enrollments`

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| student_id | bigint | 学生 ID |
| course_id | bigint | 课程 ID |
| status | tinyint | 状态，1 已选，0 已退 |
| selected_at | datetime | 选课时间 |
| dropped_at | datetime null | 退课时间 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |

索引建议：

- 唯一索引：`uk_student_course`
- 联合索引：`idx_student_status`
- 联合索引：`idx_course_status`

说明：

1. 若采用软退课模式，可保留一条记录并改 `status`。
2. 若采用硬删除模式，则退课后直接删除记录，但不利于审计。
3. 推荐软退课。

### 8.5 课程点赞表 `course_likes`

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| student_id | bigint | 学生 ID |
| course_id | bigint | 课程 ID |
| created_at | datetime | 创建时间 |

索引建议：

- 唯一索引：`uk_like_student_course`
- 普通索引：`idx_like_course_id`

### 8.6 课程评论表 `course_comments`

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| student_id | bigint | 学生 ID |
| course_id | bigint | 课程 ID |
| content | varchar(500) | 评论内容 |
| status | tinyint | 1 正常，0 已删除 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |

索引建议：

- 联合索引：`idx_comment_course_created`
- 联合索引：`idx_comment_student_created`

### 8.7 消息通知表 `notifications`

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| recipient_type | tinyint | 1 学生，2 管理员 |
| recipient_id | bigint | 接收人 ID |
| biz_type | varchar(32) | 业务类型 |
| title | varchar(128) | 标题 |
| content | varchar(500) | 内容 |
| related_course_id | bigint null | 关联课程 |
| related_comment_id | bigint null | 关联评论 |
| is_read | tinyint | 是否已读 |
| created_at | datetime | 创建时间 |

索引建议：

- 联合索引：`idx_recipient_read_created`

`biz_type` 建议值：

- `course_select_success`
- `course_drop_success`
- `comment_create_success`
- `comment_delete_success`
- `like_success`
- `unlike_success`
- `course_select_failed`

### 8.8 抢课请求表 `selection_requests`

| 字段名 | 类型 | 说明 |
| --- | --- | --- |
| id | bigint | 主键 |
| request_no | varchar(64) | 请求号，唯一 |
| student_id | bigint | 学生 ID |
| course_id | bigint | 课程 ID |
| action | varchar(16) | `grab` 或 `drop` |
| status | varchar(16) | `pending/success/failed` |
| fail_reason | varchar(255) null | 失败原因 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |

索引建议：

- 唯一索引：`uk_request_no`
- 联合索引：`idx_student_created`
- 联合索引：`idx_course_created`

### 8.9 推荐扩展表

可按项目进度补充以下表：

1. `admin_operation_logs`：管理员操作日志
2. `login_logs`：登录日志
3. `outbox_events`：本地消息表，用于更稳定的事件投递
4. `system_configs`：系统配置表

## 9. 接口设计

统一前缀建议：

```text
/api/v1
```

统一响应格式建议：

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "request_id": "202603310001"
}
```

### 9.1 认证接口

#### 1. 学生登录

- 方法：`POST`
- 路径：`/api/v1/auth/student/login`

请求体：

```json
{
  "student_no": "20230001",
  "password": "123456"
}
```

#### 2. 管理员登录

- 方法：`POST`
- 路径：`/api/v1/auth/admin/login`

请求体：

```json
{
  "admin_no": "A0001",
  "password": "123456"
}
```

#### 3. 获取当前登录用户信息

- 方法：`GET`
- 路径：`/api/v1/auth/me`

## 9.2 学生接口

#### 1. 查看个人信息

- 方法：`GET`
- 路径：`/api/v1/student/profile`

#### 2. 修改个人信息

- 方法：`PUT`
- 路径：`/api/v1/student/profile`

请求体建议：

```json
{
  "phone": "13800138000",
  "password": "new_password"
}
```

#### 3. 查看课程列表

- 方法：`GET`
- 路径：`/api/v1/student/courses`

支持查询参数：

- `page`
- `page_size`
- `course_name`
- `teacher_name`
- `status`
- `time_slot`
- `credit`

#### 4. 搜索课程

- 方法：`GET`
- 路径：`/api/v1/student/courses/search`

#### 5. 查看课程详情

- 方法：`GET`
- 路径：`/api/v1/student/courses/{courseId}`

#### 6. 课程点赞

- 方法：`POST`
- 路径：`/api/v1/student/courses/{courseId}/likes`

#### 7. 取消点赞

- 方法：`DELETE`
- 路径：`/api/v1/student/courses/{courseId}/likes`

#### 8. 获取课程评论列表

- 方法：`GET`
- 路径：`/api/v1/student/courses/{courseId}/comments`

#### 9. 发表评论

- 方法：`POST`
- 路径：`/api/v1/student/courses/{courseId}/comments`

请求体：

```json
{
  "content": "这门课很有意思"
}
```

#### 10. 删除自己的评论

- 方法：`DELETE`
- 路径：`/api/v1/student/comments/{commentId}`

#### 11. 抢课

- 方法：`POST`
- 路径：`/api/v1/student/courses/{courseId}/selections`

请求头建议：

- `X-Idempotency-Key: <uuid>`

响应建议：

```json
{
  "code": 0,
  "message": "request accepted",
  "data": {
    "request_no": "SEL202603310001",
    "status": "pending"
  }
}
```

#### 12. 查询抢课请求状态

- 方法：`GET`
- 路径：`/api/v1/student/selection-requests/{requestNo}`

#### 13. 退课

- 方法：`DELETE`
- 路径：`/api/v1/student/courses/{courseId}/selections`

#### 14. 查看课程表

- 方法：`GET`
- 路径：`/api/v1/student/timetable`

#### 15. 查看消息列表

- 方法：`GET`
- 路径：`/api/v1/student/notifications`

支持查询参数：

- `page`
- `page_size`
- `is_read`

#### 16. 标记消息已读

- 方法：`PUT`
- 路径：`/api/v1/student/notifications/{notificationId}/read`

## 9.3 管理员接口

#### 1. 查看个人信息

- 方法：`GET`
- 路径：`/api/v1/admin/profile`

#### 2. 修改个人信息

- 方法：`PUT`
- 路径：`/api/v1/admin/profile`

#### 3. 添加学生

- 方法：`POST`
- 路径：`/api/v1/admin/students`

请求体建议：

```json
{
  "student_no": "20230001",
  "password": "123456",
  "name": "张三",
  "phone": "13800138000",
  "credit_limit": 25
}
```

#### 4. 修改学生信息

- 方法：`PUT`
- 路径：`/api/v1/admin/students/{studentNo}`

可修改字段建议：

- `phone`
- `password`
- `credit_limit`
- `status`

#### 5. 根据学号查询学生信息

- 方法：`GET`
- 路径：`/api/v1/admin/students/{studentNo}`

#### 6. 新增课程

- 方法：`POST`
- 路径：`/api/v1/admin/courses`

请求体建议：

```json
{
  "course_name": "高等数学",
  "teacher_name": "李老师",
  "capacity": 80,
  "time_slot": 1,
  "credit": 4,
  "status": 1
}
```

#### 7. 修改课程信息

- 方法：`PUT`
- 路径：`/api/v1/admin/courses/{courseId}`

修改约束建议：

1. `capacity` 不得小于 `selected_count`
2. 当 `selected_count > 0` 时，`status` 不可改为 `0`
3. 当 `selected_count > 0` 时，`time_slot` 和 `credit` 默认不可修改

#### 8. 查询课程列表

- 方法：`GET`
- 路径：`/api/v1/admin/courses`

#### 9. 查询课程详情

- 方法：`GET`
- 路径：`/api/v1/admin/courses/{courseId}`

## 10. 核心接口返回错误码建议

| 错误码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 1001 | 参数错误 |
| 1002 | 未认证 |
| 1003 | 无权限 |
| 2001 | 用户不存在或密码错误 |
| 3001 | 课程不存在 |
| 3002 | 课程未开课 |
| 3003 | 课程容量已满 |
| 3004 | 已选过该课程 |
| 3005 | 课程时间冲突 |
| 3006 | 剩余学分不足 |
| 3007 | 评论不存在或无权限删除 |
| 3008 | 点赞记录不存在 |
| 3009 | 课程不可下线 |
| 3010 | 抢课请求处理中 |
| 5000 | 系统内部错误 |

## 11. 关键 SQL/事务约束建议

### 11.1 防超选更新示例

在 MySQL 事务中，更新课程已选人数时应带条件：

```sql
UPDATE courses
SET selected_count = selected_count + 1
WHERE id = ? AND status = 1 AND selected_count < capacity;
```

若影响行数为 0，则说明课程已满或不可选。

### 11.2 选课唯一约束

```sql
UNIQUE KEY uk_student_course (student_id, course_id)
```

### 11.3 点赞唯一约束

```sql
UNIQUE KEY uk_like_student_course (student_id, course_id)
```

## 12. 非功能需求

### 12.1 性能要求

1. 支持高峰期高并发抢课。
2. 抢课接口应优先走 Redis，避免直接打满 MySQL。
3. 课程列表、评论列表、消息列表必须支持分页。

### 12.2 一致性要求

1. 不允许出现超选。
2. 不允许同一学生重复选同一门课。
3. 不允许出现学分超限。
4. 不允许出现时间冲突选课成功。

### 12.3 安全要求

1. 密码必须加密存储。
2. 所有敏感接口必须鉴权。
3. 管理员接口与学生接口严格隔离。
4. 接口参数必须做长度、格式和边界校验。
5. 防止重复提交与刷接口。

### 12.4 可观测性要求

1. 记录登录日志、选课日志、管理员操作日志。
2. 记录接口耗时、错误率、MQ 堆积情况。
3. 为抢课请求分配 `request_id`，便于排查问题。

## 13. 配置项建议

建议通过 Viper 管理以下配置：

```yaml
app:
  name: choose-course
  env: dev
  port: 8080

mysql:
  host: 127.0.0.1
  port: 3306
  user: root
  password: hsp
  dbname: choose_course

redis:
  host: 127.0.0.1
  port: 6379
  password: "Cheung"
  db: 0

rabbitmq:
  host: 127.0.0.1
  port: 5672
  user: guest
  password: guest

jwt:
  secret: replace-me
  expire_hours: 24
```

## 14. 开发优先级建议

### P0

1. 用户登录与鉴权
2. 学生/管理员基础信息管理
3. 课程管理
4. 抢课核心链路
5. 退课
6. 课程表查询

### P1

1. 点赞
2. 评论
3. 消息通知
4. 操作日志

### P2

1. 监控告警
2. 死信补偿
3. 接口文档自动生成
4. 压测脚本

## 15. 测试要求建议

### 15.1 单元测试

至少覆盖以下核心逻辑：

1. 登录认证
2. 时间冲突判断
3. 学分判断
4. 抢课业务校验
5. 退课业务校验

### 15.2 集成测试

至少覆盖以下流程：

1. 学生登录 -> 查课 -> 抢课 -> 查看课表
2. 学生重复抢同一门课
3. 学生抢满课容量课程
4. 学生抢时间冲突课程
5. 学生抢学分不足课程
6. 管理员新增课程 -> 学生抢课

### 15.3 压测

建议针对抢课接口做压测，重点观察：

1. 是否超卖
2. MQ 是否积压
3. Redis 命中率
4. MySQL 事务失败率
5. 接口平均响应时间和 P99

## 16. 一期交付结果建议

一期完成后，系统应至少具备以下能力：

1. 学生与管理员登录
2. 学生查看和修改个人信息
3. 管理员新增学生、查询学生、修改学生
4. 管理员新增课程、修改课程
5. 学生查看课程、搜索课程、抢课、退课
6. 学生查看课表
7. 学生点赞、取消点赞、评论、删除评论
8. 学生查看消息通知
9. 系统在高并发抢课场景下保证不超选、不重复、无冲突、学分不超限

## 17. 后续可扩展方向

1. 支持一门课程多个时间片
2. 支持候补排队
3. 支持课程分类、院系、学期维度
4. 支持管理员批量导入学生和课程
5. 支持消息推送到 WebSocket
6. 支持分布式链路追踪和告警

