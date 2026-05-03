package middleware

import (
	"context"
	"errors"
	"net/http"

	"choose-course-backend/internal/cache"
	"choose-course-backend/internal/model"
	"choose-course-backend/internal/pkg/errno"
	authjwt "choose-course-backend/internal/pkg/jwt"
	"choose-course-backend/internal/pkg/response"
	"choose-course-backend/internal/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	errAuthUserDisabled = errors.New("auth user disabled")
	errAuthUserNotFound = errors.New("auth user not found")
	errAuthRoleInvalid  = errors.New("auth role invalid")
)

// RequireActiveUser 负责检查“当前 token 对应的账号是否仍然可用”。
//
// 它依赖 JWTAuth 先把 claims 放进 Gin 上下文，所以使用顺序必须在 JWTAuth 之后。
// 这个中间件解决的问题是：
// token 合法，不代表账号现在仍然启用。
// 比如用户昨天登录拿到了 token，今天被管理员禁用了，那么这里会拦住后续访问。
func RequireActiveUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := authjwt.FromContext(c)
		if !ok {
			response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, errno.Message(errno.CodeUnauthorized))
			c.Abort()
			return
		}

		if err := ensureActiveUser(c.Request.Context(), claims); err != nil {
			switch {
			case errors.Is(err, errAuthUserDisabled):
				response.Fail(c, http.StatusForbidden, errno.CodeUserDisabled, errno.Message(errno.CodeUserDisabled))
			case errors.Is(err, errAuthUserNotFound), errors.Is(err, errAuthRoleInvalid):
				response.Fail(c, http.StatusUnauthorized, errno.CodeUnauthorized, errno.Message(errno.CodeUnauthorized))
			default:
				response.Fail(c, http.StatusInternalServerError, errno.CodeInternalServerError, errno.Message(errno.CodeInternalServerError))
			}
			c.Abort()
			return
		}

		c.Next()
	}
}

// ensureActiveUser 根据 token 里的角色和用户 ID，
// 回数据库确认“这个账号现在是否还存在、是否仍然启用”。
func ensureActiveUser(ctx context.Context, claims *authjwt.Claims) error {
	switch claims.Role {
	case authjwt.RoleStudent, authjwt.RoleAdmin:
	default:
		return errAuthRoleInvalid
	}

	if state, hit, err := cache.GetAuthUserState(ctx, claims.Role, claims.UserID); err == nil && hit {
		return mapCachedAuthUserState(state)
	}

	switch claims.Role {
	case authjwt.RoleStudent:
		var student model.Student
		if err := repository.DB().
			WithContext(ctx).
			Select("id", "status").
			First(&student, claims.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				_ = cache.SetAuthUserState(ctx, claims.Role, claims.UserID, cache.AuthUserStateMissing)
				return errAuthUserNotFound
			}

			return err
		}
		if student.Status != 1 {
			_ = cache.SetAuthUserState(ctx, claims.Role, claims.UserID, cache.AuthUserStateDisabled)
			return errAuthUserDisabled
		}
		_ = cache.SetAuthUserState(ctx, claims.Role, claims.UserID, cache.AuthUserStateActive)
	case authjwt.RoleAdmin:
		var admin model.Admin
		if err := repository.DB().
			WithContext(ctx).
			Select("id", "status").
			First(&admin, claims.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				_ = cache.SetAuthUserState(ctx, claims.Role, claims.UserID, cache.AuthUserStateMissing)
				return errAuthUserNotFound
			}

			return err
		}
		if admin.Status != 1 {
			_ = cache.SetAuthUserState(ctx, claims.Role, claims.UserID, cache.AuthUserStateDisabled)
			return errAuthUserDisabled
		}
		_ = cache.SetAuthUserState(ctx, claims.Role, claims.UserID, cache.AuthUserStateActive)
	}

	return nil
}

func mapCachedAuthUserState(state cache.AuthUserState) error {
	switch state {
	case cache.AuthUserStateActive:
		return nil
	case cache.AuthUserStateDisabled:
		return errAuthUserDisabled
	case cache.AuthUserStateMissing:
		return errAuthUserNotFound
	default:
		return nil
	}
}
