# 高校高并发抢课系统后端开发任务清单

- 文档版本：v0.1
- 编写日期：2026-03-31
- 适用阶段：一期开发执行清单
- 关联文档：`docs/course-selection-backend-requirements.md`

## 1. 开发原则

本项目建议遵循以下实施顺序：

1. 先把基础工程和数据库结构搭好。
2. 先做同步事务版抢课，确保业务规则正确。
3. 再引入 Redis 做高并发前置校验和预扣减。
4. 最后引入 RabbitMQ 做削峰、异步落库和补偿。

原因：

1. 如果基础表结构和约束不稳，后续 Redis 与 MQ 只会放大错误。
2. 抢课项目的核心难点是业务一致性，不是组件数量。
3. 先做正确版，再做高并发版，排查问题最直接。

## 2. 里程碑总览

| 里程碑 | 名称 | 目标 |
| --- | --- | --- |
| M0 | 项目初始化 | 搭好工程骨架、配置、日志、数据库和缓存连接 |
| M1 | 数据模型落地 | 完成核心表、索引、实体和初始化数据 |
| M2 | 认证与权限 | 完成学生/管理员登录、JWT、权限隔离 |
| M3 | 管理端功能 | 完成学生管理、课程管理、管理员个人信息 |
| M4 | 学生基础功能 | 完成学生资料、课程查询、点赞、评论、消息 |
| M5 | 抢课 V1 | 完成同步事务版抢课和退课，确保业务正确 |
| M6 | 抢课 V2 | 接入 Redis，完成高并发前置校验和缓存设计 |
| M7 | 抢课 V3 | 接入 RabbitMQ，完成异步削峰、状态查询、补偿 |
| M8 | 测试与交付 | 完成测试、压测、文档、部署和监控 |

## 3. 推荐目录结构

```text
Backend/
├─ cmd/
│  └─ server/
│     └─ main.go
├─ configs/
│  └─ config.yaml
├─ docs/
├─ internal/
│  ├─ cache/
│  ├─ config/
│  ├─ handler/
│  ├─ middleware/
│  ├─ model/
│  ├─ mq/
│  ├─ pkg/
│  ├─ repository/
│  ├─ router/
│  └─ service/
├─ migrations/
├─ scripts/
├─ tests/
└─ go.mod
```

## 4. M0 项目初始化

### 4.1 目标

让项目具备可启动、可读配置、可连 MySQL/Redis、可注册路由的最小运行能力。

### 4.2 任务清单

#### 代码结构

1. 初始化 `go mod`
2. 创建基础目录结构
3. 编写启动入口 `cmd/server/main.go`
4. 抽离应用初始化流程：
   - 加载配置
   - 初始化日志
   - 初始化 MySQL
   - 初始化 Redis
   - 初始化 Gin
   - 注册路由

#### 配置管理

1. 接入 Viper
2. 定义配置结构体
3. 支持按环境加载配置
4. 提供 `config.example.yaml`

#### 基础中间件

1. 请求日志中间件
2. `recover` 中间件
3. `request_id` 中间件
4. CORS 中间件
5. 统一错误响应中间件

#### 基础工具

1. 统一响应结构
2. 错误码定义
3. 时间工具
4. ID 生成工具
5. 基础分页 DTO

### 4.3 建议生成的包/文件

- `cmd/server/main.go`
- `internal/config/config.go`
- `internal/pkg/logger/logger.go`
- `internal/pkg/response/response.go`
- `internal/pkg/errno/code.go`
- `internal/pkg/requestid/requestid.go`
- `internal/repository/mysql.go`
- `internal/repository/redis.go`
- `internal/router/router.go`
- `internal/middleware/logger.go`
- `internal/middleware/recovery.go`
- `internal/middleware/cors.go`

### 4.4 验收标准

1. 服务可启动
2. `/ping` 或 `/health` 正常返回
3. 应用启动日志可看到配置加载成功
4. MySQL、Redis 连接成功

## 5. M1 数据模型与迁移

### 5.1 目标

把需求文档中的核心表全部落到数据库，并在代码中建立实体、仓储和迁移能力。

### 5.2 任务清单

#### 核心表

1. `students`
2. `admins`
3. `courses`
4. `enrollments`
5. `course_likes`
6. `course_comments`
7. `notifications`
8. `selection_requests`

#### 数据库约束

1. 为学号、工号建立唯一索引
2. 为点赞、选课建立联合唯一索引
3. 为课程名称、教师、状态等建立查询索引
4. 为消息列表和评论列表建立分页索引

#### Model 和 Repository

1. 编写 Gorm Model
2. 抽象 Repository 接口
3. 实现 MySQL Repository
4. 补充通用查询方法：
   - 按主键查询
   - 分页查询
   - 条件查询
   - 创建
   - 更新
   - 软删除

#### 初始化数据

1. 创建默认管理员
2. 准备测试学生数据
3. 准备测试课程数据

### 5.3 建议生成的包/文件

- `internal/model/student.go`
- `internal/model/admin.go`
- `internal/model/course.go`
- `internal/model/enrollment.go`
- `internal/model/course_like.go`
- `internal/model/course_comment.go`
- `internal/model/notification.go`
- `internal/model/selection_request.go`
- `migrations/*.sql`
- `scripts/seed.sql`

### 5.4 验收标准

1. 所有核心表建成
2. 约束和索引正确
3. 能通过脚本或程序初始化测试数据
4. 基础增删改查可运行

## 6. M2 认证与权限

### 6.1 目标

完成学生与管理员登录、JWT 鉴权和角色隔离，为后续所有业务接口提供统一认证基础。

### 6.2 任务清单

#### 登录认证

1. 学生登录接口
2. 管理员登录接口
3. 密码加密存储与校验
4. JWT 签发和解析
5. 获取当前登录用户信息接口

#### 权限体系

1. 定义 `student`、`admin` 两种角色
2. 编写 JWT 鉴权中间件
3. 编写角色权限中间件
4. 路由分组：
   - 公共路由
   - 学生路由
   - 管理员路由

#### 安全细节

1. 登录失败统一错误信息
2. 登录参数校验
3. Token 过期控制
4. 可选：刷新 Token 机制

### 6.3 对应接口

1. `POST /api/v1/auth/student/login`
2. `POST /api/v1/auth/admin/login`
3. `GET /api/v1/auth/me`

### 6.4 建议生成的包/文件

- `internal/pkg/jwt/jwt.go`
- `internal/service/auth_service.go`
- `internal/handler/auth_handler.go`
- `internal/middleware/jwt.go`
- `internal/middleware/role.go`

### 6.5 验收标准

1. 学生和管理员都能成功登录
2. Token 可以正确识别用户身份和角色
3. 学生不能访问管理员接口
4. 管理员不能访问学生专属接口

## 7. M3 管理端功能

### 7.1 目标

先让管理员具备维护学生和课程数据的能力，为学生端和抢课逻辑提供稳定数据来源。

### 7.2 任务清单

#### 管理员个人信息

1. 查看个人信息
2. 修改个人信息
3. 修改密码

#### 学生管理

1. 添加学生
2. 根据学号查询学生信息
3. 修改学生信息
4. 控制可修改字段：
   - 手机号
   - 密码
   - 学分上限
   - 状态

#### 课程管理

1. 新增课程
2. 修改课程信息
3. 查看课程列表
4. 查看课程详情

#### 课程管理规则

1. `capacity` 必须大于 0
2. `credit` 只能为 2、3、4
3. `time_slot` 必须在 0-20 之间
4. `selected_count > 0` 时：
   - 课程不能下线
   - `capacity` 不能小于 `selected_count`
   - `time_slot` 默认不允许修改
   - `credit` 默认不允许修改

### 7.3 对应接口

1. `GET /api/v1/admin/profile`
2. `PUT /api/v1/admin/profile`
3. `POST /api/v1/admin/students`
4. `GET /api/v1/admin/students/{studentNo}`
5. `PUT /api/v1/admin/students/{studentNo}`
6. `POST /api/v1/admin/courses`
7. `PUT /api/v1/admin/courses/{courseId}`
8. `GET /api/v1/admin/courses`
9. `GET /api/v1/admin/courses/{courseId}`

### 7.4 建议生成的包/文件

- `internal/service/admin_service.go`
- `internal/service/student_admin_service.go`
- `internal/service/course_admin_service.go`
- `internal/handler/admin_handler.go`
- `internal/handler/admin_student_handler.go`
- `internal/handler/admin_course_handler.go`

### 7.5 验收标准

1. 管理员可以完整维护学生
2. 管理员可以完整维护课程
3. 核心业务约束全部生效
4. 错误码和错误提示统一

## 8. M4 学生基础功能

### 8.1 目标

完成学生的基础查询和互动能力，让普通业务先完整运行起来。

### 8.2 任务清单

#### 个人资料

1. 查看个人信息
2. 修改手机号
3. 修改密码
4. 明确禁止修改姓名和学号

#### 课程查询

1. 课程列表
2. 条件搜索
3. 课程详情
4. 分页
5. 课程状态过滤
6. 时间片过滤
7. 学分过滤

#### 点赞

1. 点赞课程
2. 取消点赞
3. 同一学生同一课程只能点赞一次
4. 维护 `like_count`

#### 评论

1. 获取课程评论列表
2. 发表评论
3. 删除自己的评论
4. 维护 `comment_count`
5. 评论长度限制

#### 消息中心

1. 查看消息列表
2. 标记消息已读
3. 支持按已读状态筛选

### 8.3 对应接口

1. `GET /api/v1/student/profile`
2. `PUT /api/v1/student/profile`
3. `GET /api/v1/student/courses`
4. `GET /api/v1/student/courses/search`
5. `GET /api/v1/student/courses/{courseId}`
6. `POST /api/v1/student/courses/{courseId}/likes`
7. `DELETE /api/v1/student/courses/{courseId}/likes`
8. `GET /api/v1/student/courses/{courseId}/comments`
9. `POST /api/v1/student/courses/{courseId}/comments`
10. `DELETE /api/v1/student/comments/{commentId}`
11. `GET /api/v1/student/notifications`
12. `PUT /api/v1/student/notifications/{notificationId}/read`

### 8.4 建议生成的包/文件

- `internal/service/student_profile_service.go`
- `internal/service/course_query_service.go`
- `internal/service/course_like_service.go`
- `internal/service/course_comment_service.go`
- `internal/service/notification_service.go`
- `internal/handler/student_handler.go`
- `internal/handler/course_handler.go`
- `internal/handler/comment_handler.go`
- `internal/handler/notification_handler.go`

### 8.5 验收标准

1. 学生能查看和修改个人资料
2. 学生能分页查询课程
3. 点赞和取消点赞逻辑正确
4. 评论和删除评论逻辑正确
5. 成功操作能看到对应通知

## 9. M5 抢课 V1：同步事务正确版

### 9.1 目标

先做一版不依赖 Redis 和 RabbitMQ 的同步事务版抢课逻辑，确保业务规则正确。

### 9.2 必须完成的业务规则

抢课时必须校验：

1. 课程存在
2. 课程已开课
3. 课程容量有剩余
4. 学生未选过该课
5. 与已选课程无时间冲突
6. 学生剩余学分足够

退课时必须校验：

1. 学生确实已选该课
2. 退课后课程人数回退
3. 退课后学生已用学分回退

### 9.3 任务清单

#### 选课逻辑

1. 实现抢课 Service
2. 实现退课 Service
3. 建立课程表查询接口
4. 建立抢课请求记录表写入逻辑

#### 数据库事务

1. 使用事务包裹抢课流程
2. 对课程记录加锁或使用条件更新
3. 使用唯一索引防止重复选课
4. 更新学生已用学分
5. 更新课程已选人数
6. 写入通知消息

#### 时间冲突判断

一期先按单 `time_slot` 实现：

1. 读取学生已选课程时间片集合
2. 判断新课程时间片是否已存在

#### 课程表

1. 查询学生已选课程
2. 按时间片排序返回
3. 返回课程名称、老师、学分、时间片等信息

### 9.4 对应接口

1. `POST /api/v1/student/courses/{courseId}/selections`
2. `DELETE /api/v1/student/courses/{courseId}/selections`
3. `GET /api/v1/student/timetable`
4. `GET /api/v1/student/selection-requests/{requestNo}`

### 9.5 建议生成的包/文件

- `internal/service/selection_service.go`
- `internal/service/timetable_service.go`
- `internal/handler/selection_handler.go`
- `internal/handler/timetable_handler.go`

### 9.6 验收标准

1. 单机同步模式下不超选
2. 同一学生不能重复选同一门课
3. 时间冲突课程不能选中
4. 学分不足课程不能选中
5. 退课后人数和学分正确回退

## 10. M6 抢课 V2：Redis 并发优化

### 10.1 目标

在 V1 业务正确的基础上，把高频校验前置到 Redis，降低数据库压力。

### 10.2 任务清单

#### Redis 键设计

1. `course:stock:{courseId}`
2. `course:status:{courseId}`
3. `course:credit:{courseId}`
4. `course:slot:{courseId}`
5. `student:selected:{studentId}`
6. `student:credit_used:{studentId}`
7. `student:slot_bitmap:{studentId}`

#### 缓存初始化

1. 系统启动时预热课程缓存
2. 学生登录或首次抢课时加载学生缓存
3. 课程变更后刷新课程缓存
4. 退课和抢课成功后更新学生缓存

#### Lua 脚本

1. 原子校验课程状态
2. 原子校验库存
3. 原子校验重复选课
4. 原子校验时间冲突
5. 原子校验学分额度
6. 原子预扣库存
7. 原子更新学生选课集合和学分信息

#### 回源和补建

1. 缓存不存在时回源数据库
2. 补建课程缓存
3. 补建学生缓存

### 10.3 代码拆分建议

- `internal/cache/keys.go`
- `internal/cache/course_cache.go`
- `internal/cache/student_cache.go`
- `internal/cache/lua/*.lua`
- `internal/service/selection_cache_service.go`

### 10.4 验收标准

1. 抢课请求主要命中 Redis
2. Redis 预检查逻辑正确
3. 缓存失效或丢失时可以自动恢复
4. 高并发下数据库压力明显下降

## 11. M7 抢课 V3：RabbitMQ 异步削峰

### 11.1 目标

让抢课接口从“同步事务执行”升级为“快速受理 + 异步落库”，进一步承载高并发场景。

### 11.2 任务清单

#### MQ 基础设施

1. 建立交换机
2. 建立抢课队列
3. 建立退课队列
4. 建立通知队列
5. 建立死信队列
6. 建立消费者

#### 抢课流程改造

1. 接口接收请求后生成 `request_no`
2. 调用 Redis Lua 完成前置校验和预扣减
3. 写入 `selection_requests`
4. 投递抢课消息到 MQ
5. 接口快速返回 `pending`

#### 消费端处理

1. 消费者取消息
2. 执行 MySQL 事务落库
3. 再次做关键幂等校验
4. 成功后更新 `selection_requests = success`
5. 失败后更新 `selection_requests = failed`
6. 失败后执行 Redis 补偿
7. 发送通知消息

#### 幂等性

1. `request_no` 唯一
2. `enrollments(student_id, course_id)` 唯一
3. 消费端检查请求状态，避免重复消费

### 11.3 对应接口

1. `POST /api/v1/student/courses/{courseId}/selections`
2. `GET /api/v1/student/selection-requests/{requestNo}`

### 11.4 建议生成的包/文件

- `internal/mq/rabbitmq.go`
- `internal/mq/publisher.go`
- `internal/mq/consumer.go`
- `internal/mq/message.go`
- `internal/service/selection_async_service.go`

### 11.5 验收标准

1. 抢课接口可以快速返回受理结果
2. 消费端可以稳定落库
3. 同一请求不会被重复处理
4. 异常情况下 Redis 能回滚补偿
5. 用户可以查询最终抢课结果

## 12. M8 一致性、测试与交付

### 12.1 目标

补齐可上线所需的测试、补偿、压测、部署和可观测能力。

### 12.2 任务清单

#### 一致性补偿

1. 定时扫描 `selection_requests`
2. 修复长时间 `pending` 的请求
3. 对账课程 `selected_count`
4. 对账学生 `credit_used`
5. 对账 Redis 库存与 MySQL 实际人数

#### 自动化测试

1. 单元测试：
   - 登录认证
   - 时间冲突判断
   - 学分判断
   - 点赞逻辑
   - 评论权限
   - 抢课校验
2. 集成测试：
   - 登录到抢课完整链路
   - 重复抢课
   - 满员抢课
   - 时间冲突抢课
   - 学分不足抢课
   - 退课流程

#### 压测

1. 编写抢课压测脚本
2. 观察超卖情况
3. 观察 Redis 命中率
4. 观察 MQ 堆积
5. 观察 MySQL 事务失败率

#### 交付资料

1. `README.md`
2. 配置说明
3. Swagger 文档
4. `docker-compose.yml`
5. 初始化 SQL
6. 启动说明

#### 可观测性

1. 接口访问日志
2. 错误日志
3. MQ 消费日志
4. 抢课请求追踪日志
5. Prometheus 指标接口

### 12.3 验收标准

1. 核心链路具备自动化测试
2. 高并发场景下不超卖
3. 异常请求可补偿恢复
4. 文档完整，项目可独立部署

## 13. 每个里程碑的交付物

| 里程碑 | 交付物 |
| --- | --- |
| M0 | 可启动后端、基础配置、数据库和 Redis 连接 |
| M1 | 所有核心表、索引、Model、迁移脚本、种子数据 |
| M2 | 登录接口、JWT 中间件、角色权限控制 |
| M3 | 管理员学生管理和课程管理接口 |
| M4 | 学生资料、课程查询、点赞、评论、消息接口 |
| M5 | 同步事务版抢课、退课、课表接口 |
| M6 | Redis 缓存层、Lua 校验脚本、缓存回源逻辑 |
| M7 | RabbitMQ 异步抢课、请求状态查询、补偿逻辑 |
| M8 | 测试、压测、部署文档、监控指标 |

## 14. 推荐开发顺序

建议按以下顺序推进，且每个阶段完成后再进入下一阶段：

1. M0 项目初始化
2. M1 数据模型与迁移
3. M2 认证与权限
4. M3 管理端功能
5. M4 学生基础功能
6. M5 抢课 V1
7. M6 抢课 V2
8. M7 抢课 V3
9. M8 一致性、测试与交付

## 15. 推荐提交粒度

建议按里程碑拆分 Git 提交，不建议把所有内容堆在一个大提交里。

推荐提交粒度如下：

1. `init: bootstrap project structure and config`
2. `feat: add core models and migrations`
3. `feat: implement auth and role middleware`
4. `feat: implement admin student and course management`
5. `feat: implement student profile, course query, likes and comments`
6. `feat: implement transactional course selection and drop`
7. `feat: add redis cache and lua precheck for selection`
8. `feat: add rabbitmq async selection workflow`
9. `test: add core business tests and stress scripts`
10. `docs: add deployment and api docs`

## 16. 推荐当前就开始做的第一批任务

如果现在立即开工，建议先完成下面这组最小任务：

1. 初始化 `go.mod`
2. 建目录结构
3. 接入 Viper、Gin、Gorm、Redis、Zap
4. 写 `config.yaml` 读取逻辑
5. 写 MySQL、Redis 初始化
6. 写基础路由和 `/health`
7. 创建核心表迁移脚本
8. 写默认管理员和测试课程初始化脚本

做完这批后，项目就从“只有文档”进入“可启动、可建表、可继续开发”的状态。

