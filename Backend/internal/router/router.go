package router

import (
	"github.com/gin-gonic/gin"

	"choose-course-backend/internal/handler"
	"choose-course-backend/internal/middleware"
	authjwt "choose-course-backend/internal/pkg/jwt"
	"choose-course-backend/internal/service"
)

// New 创建并返回一个配置好的 Gin 引擎。
// 当前阶段只注册了基础中间件和健康检查路由。
func New() *gin.Engine {
	engine := gin.New()

	// 中间件执行顺序很重要：
	// 1. RequestID 先生成请求编号
	// 2. Logger 记录请求日志
	// 3. Recovery 兜底 panic
	// 4. CORS 处理跨域头
	engine.Use(middleware.RequestID(), middleware.Logger(), middleware.Recovery(), middleware.CORS())

	// 根路径下保留最简单的调试接口。
	engine.GET("/ping", handler.Ping)
	engine.GET("/health", handler.Health)

	// /api/v1 是后续正式业务接口的统一前缀。
	api := engine.Group("/api/v1")
	{
		api.GET("/ping", handler.Ping)
		api.GET("/health", handler.Health)

		// 认证相关接口统一收口到 /api/v1/auth。
		// 这是 M2 新增的核心能力：
		// - 学生登录
		// - 管理员登录
		// - 获取当前登录用户信息
		authService := service.NewAuthService()
		authHandler := handler.NewAuthHandler(authService)
		auth := api.Group("/auth")
		{
			auth.POST("/student/login", authHandler.StudentLogin)
			auth.POST("/admin/login", authHandler.AdminLogin)

			// /auth/me 需要先经过 JWT 鉴权，只有带有效 token 的请求才能访问。
			authProtected := auth.Group("")
			authProtected.Use(middleware.JWTAuth(), middleware.RequireActiveUser())
			{
				authProtected.GET("/me", authHandler.Me)
			}
		}

		// 管理端接口统一挂到 /api/v1/admin 下。
		// 它们有两个共同前置条件：
		// 1. 必须先通过 JWT 鉴权
		// 2. 必须是 admin 角色
		adminService := service.NewAdminService()
		adminHandler := handler.NewAdminHandler(adminService)

		adminStudentService := service.NewAdminStudentService()
		adminStudentHandler := handler.NewAdminStudentHandler(adminStudentService)

		adminCourseService := service.NewAdminCourseService()
		adminCourseHandler := handler.NewAdminCourseHandler(adminCourseService)

		admin := api.Group("/admin")
		admin.Use(middleware.JWTAuth(), middleware.RequireActiveUser(), middleware.RequireRole(authjwt.RoleAdmin))
		{
			// 管理员个人资料接口。
			admin.GET("/profile", adminHandler.GetProfile)
			admin.PUT("/profile", adminHandler.UpdateProfile)

			// 学生管理接口。
			admin.POST("/students", adminStudentHandler.CreateStudent)
			admin.GET("/students/:studentNo", adminStudentHandler.GetStudentByNo)
			admin.PUT("/students/:studentNo", adminStudentHandler.UpdateStudentByNo)

			// 课程管理接口。
			admin.POST("/courses", adminCourseHandler.CreateCourse)
			admin.GET("/courses", adminCourseHandler.ListCourses)
			admin.GET("/courses/:courseId", adminCourseHandler.GetCourseByID)
			admin.PUT("/courses/:courseId", adminCourseHandler.UpdateCourse)
		}

		// 学生端接口统一挂在 /api/v1/student 下。
		// 这里和管理员端一样，也统一挂上 JWT 鉴权和 student 角色限制。
		studentProfileService := service.NewStudentProfileService()
		studentProfileHandler := handler.NewStudentProfileHandler(studentProfileService)

		studentCourseService := service.NewStudentCourseService()
		studentLikeService := service.NewStudentLikeService()
		studentCourseHandler := handler.NewStudentCourseHandler(studentCourseService, studentLikeService)

		studentCommentService := service.NewStudentCommentService()
		studentCommentHandler := handler.NewStudentCommentHandler(studentCommentService)

		studentNotificationService := service.NewStudentNotificationService()
		studentNotificationHandler := handler.NewStudentNotificationHandler(studentNotificationService)

		selectionService := service.NewSelectionService()
		selectionAsyncService := service.NewSelectionAsyncService()
		selectionHandler := handler.NewSelectionHandler(selectionService, selectionAsyncService)

		timetableService := service.NewTimetableService()
		timetableHandler := handler.NewTimetableHandler(timetableService)

		student := api.Group("/student")
		student.Use(middleware.JWTAuth(), middleware.RequireActiveUser(), middleware.RequireRole(authjwt.RoleStudent))
		{
			// 学生个人资料接口。
			student.GET("/profile", studentProfileHandler.GetProfile)
			student.PUT("/profile", studentProfileHandler.UpdateProfile)

			// 课程列表、搜索和详情。
			// 注意：/courses/search 一定要放在 /courses/:courseId 前面。
			student.GET("/courses", studentCourseHandler.ListCourses)
			student.GET("/courses/search", studentCourseHandler.SearchCourses)
			student.GET("/courses/:courseId", studentCourseHandler.GetCourseByID)

			// 课程点赞接口。
			student.POST("/courses/:courseId/likes", studentCourseHandler.LikeCourse)
			student.DELETE("/courses/:courseId/likes", studentCourseHandler.UnlikeCourse)

			// 课程评论接口。
			student.GET("/courses/:courseId/comments", studentCommentHandler.ListCourseComments)
			student.POST("/courses/:courseId/comments", studentCommentHandler.CreateComment)
			student.DELETE("/comments/:commentId", studentCommentHandler.DeleteOwnComment)

			// M5：同步事务版抢课、退课和课表查询。
			student.POST("/courses/:courseId/selections", selectionHandler.SelectCourse)
			student.DELETE("/courses/:courseId/selections", selectionHandler.DropCourse)
			student.GET("/timetable", timetableHandler.GetTimetable)
			student.GET("/selection-requests/:requestNo", selectionHandler.GetSelectionRequest)

			// 消息中心接口。
			student.GET("/notifications", studentNotificationHandler.ListNotifications)
			student.PUT("/notifications/:notificationId/read", studentNotificationHandler.MarkRead)
		}
	}

	return engine
}
