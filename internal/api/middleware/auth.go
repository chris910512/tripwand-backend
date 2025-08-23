package middleware

import (
	"os"
	"strings"
	"time"
	"tripwand-backend/internal/database"
	"tripwand-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// AuthMiddleware - 인증 확인 (필수) - Fiber 버전
func AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := extractTokenFiber(c)
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No token provided",
			})
		}

		claims, err := validateToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid token",
			})
		}

		// 사용자 정보를 context에 저장
		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)
		return c.Next()
	}
}

// OptionalAuthMiddleware - 인증 선택적 (비회원도 사용 가능) - Fiber 버전
func OptionalAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := extractTokenFiber(c)
		if token != "" {
			claims, err := validateToken(token)
			if err == nil {
				c.Locals("user_id", claims.UserID)
				c.Locals("email", claims.Email)
				c.Locals("is_authenticated", true)
			}
		}
		return c.Next()
	}
}

func extractTokenFiber(c *fiber.Ctx) string {
	bearerToken := c.Get("Authorization")
	if len(strings.Split(bearerToken, " ")) == 2 {
		return strings.Split(bearerToken, " ")[1]
	}
	return ""
}

func validateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, err
	}

	// 세션 확인
	var session models.UserSession
	if err := database.DB.Where("access_token = ? AND expires_at > ?", tokenString, time.Now()).First(&session).Error; err != nil {
		return nil, err
	}

	return claims, nil
}

// GenerateToken - JWT 토큰 생성
func GenerateToken(userID uint, email string) (string, string, error) {
	// Access Token (1시간)
	accessClaims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(jwtSecret)
	if err != nil {
		return "", "", err
	}

	// Refresh Token (7일)
	refreshClaims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(jwtSecret)
	if err != nil {
		return "", "", err
	}

	return accessTokenString, refreshTokenString, nil
}