package jwt

import (
	"errors"
	"fmt"
	"time"

	appconfig "choose-course-backend/internal/config"
	"github.com/gin-gonic/gin"
	gojwt "github.com/golang-jwt/jwt/v5"
)

const (
	// 这两个角色会在整个项目里复用：
	// - student：学生
	// - admin：管理员
	RoleStudent = "student"
	RoleAdmin   = "admin"

	// ContextKeyClaims 是 Gin 上下文里保存 JWT Claims 的 key。
	ContextKeyClaims = "jwt_claims"
)

var (
	// jwtConfig 保存全局 JWT 配置。
	jwtConfig appconfig.JWTConfig
	// initialized 表示 JWT 组件是否已经初始化过。
	initialized bool
)

// Claims 是我们自定义的 JWT 载荷结构。
// 你可以把它理解成“放进 token 里的用户身份信息”。
type Claims struct {
	UserID  uint64 `json:"user_id"`  // 当前登录用户的数据库主键 ID
	Role    string `json:"role"`     // 当前登录用户的角色：student/admin
	LoginNo string `json:"login_no"` // 学号或工号，便于后续快速识别
	gojwt.RegisteredClaims
}

// Init 初始化 JWT 配置。
// 程序启动时会调用它，把 config.yaml 里的 jwt 配置加载进来。
func Init(cfg appconfig.JWTConfig) error {
	if cfg.Secret == "" {
		return errors.New("jwt secret cannot be empty")
	}
	if cfg.ExpireHours <= 0 {
		return errors.New("jwt expire hours must be greater than 0")
	}

	jwtConfig = cfg
	initialized = true
	return nil
}

// GenerateToken 根据用户身份信息生成一个 JWT。
func GenerateToken(userID uint64, role, loginNo string) (string, time.Time, error) {
	if !initialized {
		return "", time.Time{}, errors.New("jwt not initialized")
	}

	expireAt := time.Now().Add(time.Duration(jwtConfig.ExpireHours) * time.Hour)
	claims := Claims{
		UserID:  userID,
		Role:    role,
		LoginNo: loginNo,
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
			NotBefore: gojwt.NewNumericDate(time.Now()),
			ExpiresAt: gojwt.NewNumericDate(expireAt),
		},
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtConfig.Secret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expireAt, nil
}

// ParseToken 把 token 字符串解析回 Claims。
func ParseToken(tokenString string) (*Claims, error) {
	if !initialized {
		return nil, errors.New("jwt not initialized")
	}

	token, err := gojwt.ParseWithClaims(tokenString, &Claims{}, func(token *gojwt.Token) (any, error) {
		if token.Method != gojwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}

		return []byte(jwtConfig.Secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// FromContext 从 Gin 上下文中取出 JWT Claims。
func FromContext(c *gin.Context) (*Claims, bool) {
	value, exists := c.Get(ContextKeyClaims)
	if !exists {
		return nil, false
	}

	claims, ok := value.(*Claims)
	return claims, ok
}
