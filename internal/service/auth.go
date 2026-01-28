package service

import (
	"context"
	"errors"
	"fmt"
	"mizuflow/internal/dto/req"
	"mizuflow/internal/dto/resp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	RefreshTokenTTL = 7 * 24 * time.Hour
	AccessTokenTTL  = 15 * time.Minute
	RedisKeyPrefix  = "mizuflow:auth:session:"
	Issuer          = "mizuflow-auth-service"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenInvalid       = errors.New("token invalid")
	ErrSessionExpired     = errors.New("session expired")
)

// SignedKey should be loaded from env in production
var SignedKey = []byte("mizuflow-super-secret-key-2026")

type AuthService struct {
	redis           *redis.Client
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

type UserClaims struct {
	UserID   string `json:"uid"`
	Username string `json:"sub"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func NewAuthService(rdb *redis.Client, accessTokenTTL, refreshTokenTTL time.Duration) *AuthService {
	return &AuthService{
		redis:           rdb,
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}
}

// Login authenticates a user and returns pair of tokens
func (s *AuthService) Login(ctx context.Context, req req.LoginReq) (*resp.TokenResp, error) {
	// Mock DB check
	// In real world: userRepo.FindByUsername(...)
	if req.Username != "admin" || req.Password != "admin123" {
		return nil, ErrInvalidCredentials
	}

	userID := "1001" // Mock ID
	role := "admin"

	tokens, err := s.generateTokens(ctx, userID, req.Username, role)
	if err != nil {
		return nil, err
	}
	tokens.User = resp.UserInfo{
		ID:       userID,
		Username: req.Username,
		Role:     role,
	}
	return tokens, nil
}

// Refresh handles token rotation using the Refresh Token
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*resp.TokenResp, error) {

	token, err := jwt.ParseWithClaims(refreshToken, &UserClaims{}, func(t *jwt.Token) (interface{}, error) {
		return SignedKey, nil
	})

	if err != nil {
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	key := fmt.Sprintf("%s%s", RedisKeyPrefix, claims.UserID)
	storedToken, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, ErrSessionExpired
	}
	if err != nil {
		return nil, err
	}

	if storedToken != refreshToken {
		// Token rotation reuse detection could go here (delete session if mismatch)
		return nil, ErrTokenInvalid
	}

	return s.generateTokens(ctx, claims.UserID, claims.Username, claims.Role)
}

func (s *AuthService) Logout(ctx context.Context, userID string) error {
	key := fmt.Sprintf("%s%s", RedisKeyPrefix, userID)
	return s.redis.Del(ctx, key).Err()
}

func (s *AuthService) generateTokens(ctx context.Context, userID, username, role string) (*resp.TokenResp, error) {
	// 1. Create Access Token
	now := time.Now()
	atClaims := UserClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    Issuer,
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims).SignedString(SignedKey)
	if err != nil {
		return nil, err
	}

	// 2. Create Refresh Token (Also JWT to carry UserID, but longer life)
	// Alternatively, can be opaque if we require userID in request. Let's use JWT for self-containedness.
	rtClaims := UserClaims{
		UserID:   userID,
		Username: username,
		Role:     role, // optionally less info
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    Issuer,
			ID:        uuid.New().String(), // JTI
		},
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims).SignedString(SignedKey)
	if err != nil {
		return nil, err
	}

	// 3. Store Refresh Token in Redis (Allow-list)
	key := fmt.Sprintf("%s%s", RedisKeyPrefix, userID)
	if err := s.redis.Set(ctx, key, refreshToken, s.refreshTokenTTL).Err(); err != nil {
		return nil, err
	}

	return &resp.TokenResp{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTokenTTL.Seconds()),
	}, nil
}
