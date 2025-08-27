package services

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"tripwand-backend/internal/database"
	"tripwand-backend/internal/models"
)

// MessageService 채팅 메시지 관리 서비스
type MessageService struct {
	db *gorm.DB
}

// NewMessageService 새로운 메시지 서비스 생성
func NewMessageService() *MessageService {
	return &MessageService{
		db: database.DB,
	}
}

// SaveChatMessage 채팅 메시지 저장
func (s *MessageService) SaveChatMessage(roomID uint, content string, userID *uint, sessionID *string, nickname string) (*models.ChatMessage, error) {
	log.Printf("[DEBUG-SAVE] SaveChatMessage called with params:")
	log.Printf("[DEBUG-SAVE] - RoomID: %d", roomID)
	log.Printf("[DEBUG-SAVE] - Content: %s", content)
	log.Printf("[DEBUG-SAVE] - UserID: %v", userID)
	log.Printf("[DEBUG-SAVE] - SessionID: %v", sessionID)
	log.Printf("[DEBUG-SAVE] - Nickname: %s", nickname)

	// UTC 시간 사용 (파티션 테이블 호환)
	now := time.Now().UTC()

	chatMessage := &models.ChatMessage{
		// ID는 auto increment이므로 설정하지 않음
		RoomID:      roomID,
		UserID:      userID,
		SessionID:   sessionID,
		Content:     content,
		Sender:      nickname,
		MessageType: models.MessageTypeText,
		CreatedAt:   now,
	}

	log.Printf("[DEBUG-SAVE] Created ChatMessage struct: %+v", chatMessage)
	log.Printf("[DEBUG-SAVE] Using UTC time: %s", now.Format("2006-01-02 15:04:05 MST"))

	// 필요한 파티션이 존재하는지 확인하고 없으면 생성
	if err := s.ensurePartitionExists(now); err != nil {
		log.Printf("[DEBUG-SAVE] WARNING: Failed to ensure partition exists: %v", err)
		// 파티션 생성 실패해도 계속 진행 (기존 파티션이 있을 수 있음)
	}

	log.Printf("[DEBUG-SAVE] Attempting DB Create operation...")
	result := s.db.Create(chatMessage)
	if result.Error != nil {
		log.Printf("[DEBUG-SAVE] ERROR: DB Create failed: %v", result.Error)
		log.Printf("[DEBUG-SAVE] ERROR Details - SQL State: %T", result.Error)

		// 파티션 관련 에러일 경우 추가 정보 출력
		if fmt.Sprintf("%v", result.Error) != "" {
			log.Printf("[DEBUG-SAVE] Checking if partition exists for date: %s", now.Format("2006-01-02"))
			s.checkPartitionStatus(now)
		}

		return nil, fmt.Errorf("failed to save chat message: %w", result.Error)
	}

	log.Printf("[DEBUG-SAVE] SUCCESS: DB Create completed")
	log.Printf("[DEBUG-SAVE] - RowsAffected: %d", result.RowsAffected)
	log.Printf("[DEBUG-SAVE] - Final message ID: %d", chatMessage.ID)
	log.Printf("Saved message %d from %s in room %d", chatMessage.ID, nickname, roomID)
	return chatMessage, nil
}

// ensurePartitionExists 필요한 파티션이 존재하는지 확인하고 없으면 생성
func (s *MessageService) ensurePartitionExists(date time.Time) error {
	dateStr := date.Format("2006_01_02")
	tableName := fmt.Sprintf("chat_messages_%s", dateStr)

	// 파티션 테이블 존재 확인
	var exists bool
	checkSQL := `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = ? AND table_schema = 'public')`
	if err := s.db.Raw(checkSQL, tableName).Scan(&exists).Error; err != nil {
		return fmt.Errorf("failed to check partition existence: %w", err)
	}

	if exists {
		log.Printf("[DEBUG-PARTITION] Partition %s already exists", tableName)
		return nil
	}

	// 파티션 생성
	log.Printf("[DEBUG-PARTITION] Creating missing partition: %s", tableName)
	startDate := date.Format("2006-01-02")
	endDate := date.AddDate(0, 0, 1).Format("2006-01-02")

	partitionSQL := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s PARTITION OF chat_messages
	FOR VALUES FROM ('%s 00:00:00+00') TO ('%s 00:00:00+00');`,
		tableName, startDate, endDate)

	if err := s.db.Exec(partitionSQL).Error; err != nil {
		return fmt.Errorf("failed to create partition %s: %w", tableName, err)
	}

	log.Printf("[DEBUG-PARTITION] Successfully created partition: %s", tableName)
	return nil
}

// checkPartitionStatus 파티션 상태 확인 (디버깅용)
func (s *MessageService) checkPartitionStatus(date time.Time) {
	log.Printf("[DEBUG-PARTITION] === Partition Status Check ===")

	// 현재 서버 시간 및 시간대 확인
	var serverTime, utcTime, timezone string
	err := s.db.Raw("SELECT current_timestamp, timezone('UTC', current_timestamp), current_setting('timezone')").
		Row().Scan(&serverTime, &utcTime, &timezone)
	if err != nil {
		return
	}
	log.Printf("[DEBUG-PARTITION] Server time: %s, UTC: %s, Timezone: %s", serverTime, utcTime, timezone)

	// 파티션 테이블 목록
	var partitions []string
	s.db.Raw(`SELECT tablename FROM pg_tables WHERE tablename LIKE 'chat_messages_%' ORDER BY tablename`).
		Pluck("tablename", &partitions)
	log.Printf("[DEBUG-PARTITION] Existing partitions: %v", partitions)

	// 메인 테이블 존재 확인
	var mainTableExists bool
	s.db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'chat_messages' AND table_schema = 'public')`).
		Scan(&mainTableExists)
	log.Printf("[DEBUG-PARTITION] Main table exists: %v", mainTableExists)
}

// GetRoomMessages 채팅방 메시지 목록 조회 (페이징)
func (s *MessageService) GetRoomMessages(roomID uint, page, limit int) ([]*models.ChatMessage, int64, error) {
	var messages []*models.ChatMessage
	var total int64

	offset := (page - 1) * limit

	// 전체 개수 조회
	if err := s.db.Model(&models.ChatMessage{}).Where("room_id = ? AND deleted_at IS NULL", roomID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 메시지 조회 (최신순)
	result := s.db.Where("room_id = ? AND deleted_at IS NULL", roomID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&messages)

	if result.Error != nil {
		return nil, 0, result.Error
	}

	return messages, total, nil
}

// GetRoomMessagesByCountryCode 국가 코드로 메시지 조회
func (s *MessageService) GetRoomMessagesByCountryCode(countryCode string, page, limit int) ([]*models.ChatMessage, int64, error) {
	// 먼저 해당 국가의 채팅방을 찾기
	var room models.ChatRoom
	result := s.db.Where("room_type = ? AND country_code = ?", models.RoomTypePublic, countryCode).First(&room)
	if result.Error != nil {
		return nil, 0, fmt.Errorf("room not found for country %s: %w", countryCode, result.Error)
	}

	return s.GetRoomMessages(room.ID, page, limit)
}

// DeleteExpiredMessages 만료된 메시지 삭제 (24시간 후)
func (s *MessageService) DeleteExpiredMessages() error {
	// 24시간 이전 메시지들을 소프트 삭제
	expiredTime := time.Now().Add(-24 * time.Hour)

	result := s.db.Model(&models.ChatMessage{}).
		Where("created_at < ? AND deleted_at IS NULL", expiredTime).
		Update("deleted_at", time.Now())

	if result.Error != nil {
		return result.Error
	}

	log.Printf("Marked %d expired messages for deletion", result.RowsAffected)
	return nil
}

// GetMessageStats 메시지 통계 조회
func (s *MessageService) GetMessageStats() (*MessageStats, error) {
	var stats MessageStats
	now := time.Now()

	// 전체 메시지 수
	if err := s.db.Model(&models.ChatMessage{}).Where("deleted_at IS NULL").Count(&stats.TotalMessages).Error; err != nil {
		return nil, err
	}

	// 오늘 메시지 수
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if err := s.db.Model(&models.ChatMessage{}).
		Where("created_at >= ? AND deleted_at IS NULL", todayStart).
		Count(&stats.TodayMessages).Error; err != nil {
		return nil, err
	}

	// 최근 1시간 메시지 수
	hourAgo := now.Add(-1 * time.Hour)
	if err := s.db.Model(&models.ChatMessage{}).
		Where("created_at >= ? AND deleted_at IS NULL", hourAgo).
		Count(&stats.LastHourMessages).Error; err != nil {
		return nil, err
	}

	// 방별 메시지 수
	var roomStats []RoomMessageStat
	if err := s.db.Model(&models.ChatMessage{}).
		Select("room_id, COUNT(*) as message_count").
		Where("deleted_at IS NULL").
		Group("room_id").
		Find(&roomStats).Error; err != nil {
		return nil, err
	}

	stats.MessagesByRoom = roomStats

	return &stats, nil
}

// BlockMessage 메시지 차단 처리
func (s *MessageService) BlockMessage(messageID, reason string) error {
	result := s.db.Model(&models.ChatMessage{}).
		Where("id = ?", messageID).
		Updates(map[string]interface{}{
			"message_type":   models.MessageTypeBlocked,
			"blocked_reason": reason,
			"blocked_at":     time.Now(),
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("message not found: %s", messageID)
	}

	log.Printf("Blocked message %s: %s", messageID, reason)
	return nil
}

// MessageStats 메시지 통계 구조체
type MessageStats struct {
	TotalMessages    int64             `json:"total_messages"`
	TodayMessages    int64             `json:"today_messages"`
	LastHourMessages int64             `json:"last_hour_messages"`
	MessagesByRoom   []RoomMessageStat `json:"messages_by_room"`
}

// RoomMessageStat 방별 메시지 통계
type RoomMessageStat struct {
	RoomID       uint  `json:"room_id"`
	MessageCount int64 `json:"message_count"`
}

// GetRoomByCountryCode 국가 코드로 채팅방 조회
func (s *MessageService) GetRoomByCountryCode(countryCode string) (*models.ChatRoom, error) {
	var room models.ChatRoom
	result := s.db.Where("room_type = ? AND country_code = ? AND deleted_at IS NULL",
		models.RoomTypePublic, countryCode).First(&room)

	if result.Error != nil {
		return nil, result.Error
	}

	return &room, nil
}

// CleanupOldMessages 오래된 메시지 물리적 삭제 (성능 최적화)
func (s *MessageService) CleanupOldMessages() error {
	// 7일 전에 소프트 삭제된 메시지들을 물리적으로 삭제
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)

	result := s.db.Unscoped().Where("deleted_at < ?", sevenDaysAgo).Delete(&models.ChatMessage{})
	if result.Error != nil {
		return result.Error
	}

	log.Printf("Permanently deleted %d old messages", result.RowsAffected)
	return nil
}
