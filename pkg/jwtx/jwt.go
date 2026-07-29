package jwtx

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/leafney/filedock/pkg/cachex"
)

// JWTConfig JWT 配置接口
type JWTConfig interface {
	GetJWTSigningKey() string
	GetAccessTokenExpire() int  // 秒
	GetRefreshTokenExpire() int // 秒
}

// Claims JWT 载荷
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	UserType string `json:"user_type"`
	jwt.RegisteredClaims
}

// TokenType 令牌类型
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// JWTService JWT 服务
type JWTService struct {
	signingKey         []byte
	accessTokenExpire  time.Duration
	refreshTokenExpire time.Duration
	cache              *cachex.CacheSvc
}

// NewJWTService 创建 JWT 服务
func NewJWTService(cfg JWTConfig, cache *cachex.CacheSvc) *JWTService {
	return &JWTService{
		signingKey:         []byte(cfg.GetJWTSigningKey()),
		accessTokenExpire:  time.Duration(cfg.GetAccessTokenExpire()) * time.Second,
		refreshTokenExpire: time.Duration(cfg.GetRefreshTokenExpire()) * time.Second,
		cache:              cache,
	}
}

// GenerateTokenPair 生成 Access Token 和 Refresh Token 对
func (j *JWTService) GenerateTokenPair(userID string, username, role, userType string) (accessToken, refreshToken string, err error) {
	// 生成 Access Token
	accessToken, err = j.generateToken(userID, username, role, userType, AccessToken)
	if err != nil {
		return "", "", err
	}

	// 生成 Refresh Token
	refreshToken, err = j.generateToken(userID, username, role, userType, RefreshToken)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// generateToken 生成指定类型的 Token
func (j *JWTService) generateToken(userID string, username, role, userType string, tokenType TokenType) (string, error) {
	var expireTime time.Duration
	if tokenType == AccessToken {
		expireTime = j.accessTokenExpire
	} else {
		expireTime = j.refreshTokenExpire
	}

	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		UserType: userType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expireTime)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "Aegis",
			Subject:   string(tokenType),
			ID:        fmt.Sprintf("%s-%d", userID, now.UnixNano()), // 为每个 token 添加唯一标识
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.signingKey)
}

// ParseToken 解析并验证 Token
func (j *JWTService) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return j.signingKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		// 检查是否在黑名单中
		if j.IsTokenBlacklisted(claims.ID) {
			return nil, errors.New("token in blacklist")
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// RefreshAccessToken 使用 Refresh Token 刷新 Access Token
func (j *JWTService) RefreshAccessToken(refreshTokenString string) (newAccessToken string, err error) {
	// 解析 Refresh Token
	claims, err := j.ParseToken(refreshTokenString)
	if err != nil {
		return "", err
	}

	// 验证是否为 Refresh Token
	if claims.Subject != string(RefreshToken) {
		return "", errors.New("not a refresh token")
	}

	// 生成新的 Access Token
	return j.generateToken(claims.UserID, claims.Username, claims.Role, claims.UserType, AccessToken)
}

// ValidateTokenType 验证 Token 类型
func (j *JWTService) ValidateTokenType(tokenString string, expectedType TokenType) error {
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return err
	}

	if claims.Subject != string(expectedType) {
		return errors.New("token type mismatch")
	}

	return nil
}

// BlacklistToken 将 Token 加入黑名单
func (j *JWTService) BlacklistToken(claims *Claims) error {
	if j.cache == nil || claims == nil || claims.ID == "" {
		return nil
	}
	// 距离过期的剩余时间
	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining <= 0 {
		return nil
	}

	// 设置到缓存中，过期时间为 token 的剩余过期时间
	key := fmt.Sprintf("blacklist:%s", claims.ID)
	return j.cache.XSetExS(key, "1", remaining)
}

// IsTokenBlacklisted 检查 Token 是否在黑名单中
func (j *JWTService) IsTokenBlacklisted(tokenID string) bool {
	if j.cache == nil || tokenID == "" {
		return false
	}
	key := fmt.Sprintf("blacklist:%s", tokenID)
	return j.cache.Exists(key)
}
