package database

import (
	"fmt"
	"log"
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
	// 기존 테이블 마이그레이션
	if err := migrateUserTables(); err != nil {
		return fmt.Errorf("failed to migrate user tables: %w", err)
	}

	// 채팅 테이블 마이그레이션
	if err := migrateChatTables(); err != nil {
		return fmt.Errorf("failed to migrate chat tables: %w", err)
	}

	return nil
}

// migrateUserTables 사용자 관련 테이블 마이그레이션
func migrateUserTables() error {
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

// migrateChatTables 채팅 관련 테이블 마이그레이션
func migrateChatTables() error {
	log.Println("Starting chat tables migration...")

	// 1. 익명 세션 테이블 먼저 생성
	log.Println("Creating anonymous_sessions table...")
	if err := DB.AutoMigrate(&models.AnonymousSession{}); err != nil {
		return fmt.Errorf("failed to migrate anonymous_sessions table: %w", err)
	}

	// 2. 채팅룸 테이블 생성
	log.Println("Creating chat_rooms table...")
	if err := DB.AutoMigrate(&models.ChatRoom{}); err != nil {
		return fmt.Errorf("failed to migrate chat_rooms table: %w", err)
	}

	// 3. 파티션 매니저로 채팅 메시지 파티션 테이블 생성 (GORM 외부에서 직접 생성)
	log.Println("Creating chat_messages partitioned table...")
	partitionManager := NewPartitionManager(DB)
	if err := partitionManager.CreateChatMessagesPartitions(); err != nil {
		return fmt.Errorf("failed to create chat messages partitions: %w", err)
	}

	// 4. 기본 채팅룸 생성 (7개 국가)
	log.Println("Creating default chat rooms...")
	if err := createDefaultChatRooms(); err != nil {
		return fmt.Errorf("failed to create default chat rooms: %w", err)
	}

	// 5. 추가 인덱스 및 제약조건 생성
	log.Println("Creating additional indexes and constraints...")
	if err := createChatIndexes(); err != nil {
		return fmt.Errorf("failed to create chat indexes: %w", err)
	}

	log.Println("Chat tables migration completed successfully")
	return nil
}

// createDefaultChatRooms 기본 공개 채팅룸 생성
func createDefaultChatRooms() error {
	for _, countryCode := range models.SupportedCountries {
		// 이미 존재하는지 확인
		var existingRoom models.ChatRoom
		result := DB.Where("room_type = ? AND country_code = ?", models.RoomTypePublic, countryCode).First(&existingRoom)
		
		if result.Error == nil {
			// 이미 존재함
			continue
		}

		// 새 채팅룸 생성
		room := models.ChatRoom{
			RoomType:    models.RoomTypePublic,
			CountryCode: &countryCode,
		}

		if err := DB.Create(&room).Error; err != nil {
			return fmt.Errorf("failed to create chat room for %s: %w", countryCode, err)
		}

		log.Printf("Created chat room for country: %s (ID: %d)", countryCode, room.ID)
	}

	return nil
}

// createChatIndexes 채팅 테이블 추가 인덱스 및 제약조건 생성
func createChatIndexes() error {
	log.Println("Creating additional chat indexes and constraints...")

	// ChatRoom 테이블 인덱스
	chatRoomIndexes := []string{
		// 공개 채팅룸의 중복 방지를 위한 유니크 인덱스
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_rooms_public_country ON chat_rooms (country_code) WHERE room_type = 'public' AND deleted_at IS NULL;",
		
		// 1:1 채팅룸의 중복 방지를 위한 유니크 인덱스 (양방향 고려)
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_rooms_private_participants ON chat_rooms (LEAST(participant1, participant2), GREATEST(participant1, participant2)) WHERE room_type = 'private' AND deleted_at IS NULL;",
		
		// 룸 타입별 검색 인덱스
		"CREATE INDEX IF NOT EXISTS idx_chat_rooms_room_type ON chat_rooms (room_type) WHERE deleted_at IS NULL;",
		
		// 업데이트 시간 인덱스 (최근 활성 채팅룸 조회용)
		"CREATE INDEX IF NOT EXISTS idx_chat_rooms_updated_at ON chat_rooms (updated_at DESC) WHERE deleted_at IS NULL;",
	}

	// AnonymousSession 테이블 인덱스
	anonymousSessionIndexes := []string{
		// 브라우저 지문으로 세션 찾기
		"CREATE INDEX IF NOT EXISTS idx_anonymous_sessions_fingerprint ON anonymous_sessions (browser_fingerprint);",
		
		// localStorage 키로 세션 찾기
		"CREATE INDEX IF NOT EXISTS idx_anonymous_sessions_local_storage_key ON anonymous_sessions (local_storage_key);",
		
		// 만료된 세션 정리용 인덱스
		"CREATE INDEX IF NOT EXISTS idx_anonymous_sessions_expires_at ON anonymous_sessions (expires_at);",
		
		// 활성 세션 조회용 인덱스 (expires_at 기준으로 정렬)
		"CREATE INDEX IF NOT EXISTS idx_anonymous_sessions_active ON anonymous_sessions (expires_at DESC, created_at);",
	}

	// 모든 인덱스 생성
	allIndexes := append(chatRoomIndexes, anonymousSessionIndexes...)
	
	for _, indexSQL := range allIndexes {
		if err := DB.Exec(indexSQL).Error; err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// 추가 제약조건 생성
	constraints := []string{
		// ChatRoom 제약조건: 공개방은 country_code 필수, 개인방은 participants 필수
		"ALTER TABLE chat_rooms ADD CONSTRAINT IF NOT EXISTS check_public_room_country CHECK ((room_type = 'public' AND country_code IS NOT NULL) OR room_type = 'private');",
		"ALTER TABLE chat_rooms ADD CONSTRAINT IF NOT EXISTS check_private_room_participants CHECK ((room_type = 'private' AND participant1 IS NOT NULL AND participant2 IS NOT NULL) OR room_type = 'public');",
		"ALTER TABLE chat_rooms ADD CONSTRAINT IF NOT EXISTS check_participants_different CHECK (participant1 != participant2);",

		// AnonymousSession 제약조건
		"ALTER TABLE anonymous_sessions ADD CONSTRAINT IF NOT EXISTS check_expires_after_created CHECK (expires_at > created_at);",
		"ALTER TABLE anonymous_sessions ADD CONSTRAINT IF NOT EXISTS check_nickname_format CHECK (nickname ~ '^익명_[a-zA-Z0-9]{8}$');",
	}

	for _, constraintSQL := range constraints {
		// 제약조건은 이미 존재할 수 있으므로 에러 무시
		DB.Exec(constraintSQL)
	}

	log.Println("Chat indexes and constraints created successfully")
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
