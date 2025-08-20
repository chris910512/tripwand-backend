package database

import (
	"fmt"
	"os"
	"time"
	"tripwand-backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// DatabaseConfig 데이터베이스 설정 구조체
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// Connect PostgreSQL 데이터베이스 연결
func Connect() error {
	config := DatabaseConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "password"),
		DBName:   getEnv("DB_NAME", "webservice_db"),
		SSLMode:  getEnv("DB_SSL_MODE", "disable"),
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		config.Host, config.User, config.Password, config.DBName, config.Port, config.SSLMode)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 연결 풀 설정
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// pgvector 확장 활성화
	if err := enablePgVector(); err != nil {
		return fmt.Errorf("failed to enable pgvector: %w", err)
	}

	return nil
}

// enablePgVector pgvector 확장 활성화
func enablePgVector() error {
	// pgvector 확장 생성 (이미 존재하면 무시)
	result := DB.Exec("CREATE EXTENSION IF NOT EXISTS vector")
	if result.Error != nil {
		return fmt.Errorf("failed to create vector extension: %w", result.Error)
	}

	return nil
}

// 환경 변수 헬퍼 함수
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Migrate 데이터베이스 마이그레이션 실행
func Migrate() error {
	// 사용자 테이블 마이그레이션
	if err := DB.AutoMigrate(&models.User{}); err != nil {
		return fmt.Errorf("failed to migrate users table: %w", err)
	}

	// OAuth 계정 테이블 마이그레이션
	if err := DB.AutoMigrate(&models.OAuthAccount{}); err != nil {
		return fmt.Errorf("failed to migrate oauth_accounts table: %w", err)
	}

	// 사용자 세션 테이블 마이그레이션
	if err := DB.AutoMigrate(&models.UserSession{}); err != nil {
		return fmt.Errorf("failed to migrate user_sessions table: %w", err)
	}

	// 여행 계획 테이블 마이그레이션
	if err := DB.AutoMigrate(&models.TravelPlans{}); err != nil {
		return fmt.Errorf("failed to migrate travel_plans table: %w", err)
	}

	return nil
}

// Close 데이터베이스 연결 종료
func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
