package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PartitionManager 파티션 관리자
type PartitionManager struct {
	db *gorm.DB
}

// NewPartitionManager 파티션 관리자 생성
func NewPartitionManager(db *gorm.DB) *PartitionManager {
	return &PartitionManager{db: db}
}

// CreateChatMessagesPartitions 채팅 메시지 파티션 테이블 생성
func (pm *PartitionManager) CreateChatMessagesPartitions() error {
	// 메인 파티션 테이블 생성
	createMainTableSQL := `
	CREATE TABLE IF NOT EXISTS chat_messages (
		id BIGSERIAL NOT NULL,
		room_id BIGINT NOT NULL,
		user_id BIGINT,
		session_id VARCHAR(36),
		content TEXT NOT NULL,
		message_type VARCHAR(20) NOT NULL DEFAULT 'text',
		moderated_at TIMESTAMPTZ,
		is_harmful BOOLEAN,
		created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		deleted_at TIMESTAMPTZ,
		CONSTRAINT chat_messages_pkey PRIMARY KEY (id, created_at),
		CONSTRAINT chat_messages_message_type_check CHECK (message_type IN ('text', 'blocked', 'system'))
	) PARTITION BY RANGE (created_at);`

	if err := pm.db.Exec(createMainTableSQL).Error; err != nil {
		return fmt.Errorf("failed to create partitioned chat_messages table: %w", err)
	}

	// 인덱스 생성
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_chat_messages_room_id ON chat_messages (room_id);",
		"CREATE INDEX IF NOT EXISTS idx_chat_messages_user_id ON chat_messages (user_id) WHERE user_id IS NOT NULL;",
		"CREATE INDEX IF NOT EXISTS idx_chat_messages_session_id ON chat_messages (session_id) WHERE session_id IS NOT NULL;",
		"CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at ON chat_messages (created_at);",
		"CREATE INDEX IF NOT EXISTS idx_chat_messages_deleted_at ON chat_messages (deleted_at) WHERE deleted_at IS NOT NULL;",
	}

	for _, indexSQL := range indexes {
		if err := pm.db.Exec(indexSQL).Error; err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// 오늘부터 3일치 파티션 생성
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		date := now.AddDate(0, 0, i)
		if err := pm.createDailyPartition(date); err != nil {
			return fmt.Errorf("failed to create partition for %s: %w", date.Format("2006-01-02"), err)
		}
	}

	return nil
}

// createDailyPartition 일별 파티션 생성
func (pm *PartitionManager) createDailyPartition(date time.Time) error {
	dateStr := date.Format("2006_01_02")
	tableName := fmt.Sprintf("chat_messages_%s", dateStr)
	
	startDate := date.Format("2006-01-02")
	endDate := date.AddDate(0, 0, 1).Format("2006-01-02")

	partitionSQL := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s PARTITION OF chat_messages
	FOR VALUES FROM ('%s 00:00:00+00') TO ('%s 00:00:00+00');`,
		tableName, startDate, endDate)

	if err := pm.db.Exec(partitionSQL).Error; err != nil {
		return fmt.Errorf("failed to create partition %s: %w", tableName, err)
	}

	return nil
}

// CreateTomorrowPartition 내일 파티션 생성 (배치 작업용)
func (pm *PartitionManager) CreateTomorrowPartition() error {
	tomorrow := time.Now().UTC().AddDate(0, 0, 1)
	return pm.createDailyPartition(tomorrow)
}

// CleanupOldPartitions 오래된 파티션 삭제 (24시간 TTL)
func (pm *PartitionManager) CleanupOldPartitions() error {
	// 2일 전 파티션 삭제
	cutoffDate := time.Now().UTC().AddDate(0, 0, -2)
	dateStr := cutoffDate.Format("2006_01_02")
	tableName := fmt.Sprintf("chat_messages_%s", dateStr)

	// 파티션 존재 확인
	var exists bool
	checkSQL := `
	SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = ? AND table_schema = 'public'
	);`
	
	if err := pm.db.Raw(checkSQL, tableName).Scan(&exists).Error; err != nil {
		return fmt.Errorf("failed to check partition existence: %w", err)
	}

	if exists {
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s;", tableName)
		if err := pm.db.Exec(dropSQL).Error; err != nil {
			return fmt.Errorf("failed to drop partition %s: %w", tableName, err)
		}
	}

	return nil
}

// GetActivePartitions 활성 파티션 목록 조회
func (pm *PartitionManager) GetActivePartitions() ([]string, error) {
	var partitions []string
	
	querySQL := `
	SELECT schemaname||'.'||tablename as partition_name
	FROM pg_tables 
	WHERE tablename LIKE 'chat_messages_%' 
	AND schemaname = 'public'
	ORDER BY tablename;`

	if err := pm.db.Raw(querySQL).Scan(&partitions).Error; err != nil {
		return nil, fmt.Errorf("failed to get active partitions: %w", err)
	}

	return partitions, nil
}