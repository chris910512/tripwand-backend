package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tripwand-backend/internal/database"
	"tripwand-backend/internal/models"
)

// SessionService 익명 세션 관리 서비스
type SessionService struct {
	db *gorm.DB
}

// NewSessionService 새로운 세션 서비스 생성
func NewSessionService() *SessionService {
	return &SessionService{
		db: database.DB,
	}
}

// CreateOrRecoverSession 세션 생성 또는 복구
func (s *SessionService) CreateOrRecoverSession(browserFingerprint, localStorageKey string) (*models.AnonymousSession, error) {
	now := time.Now()

	// 1. localStorage 키로 기존 세션 찾기 시도
	if localStorageKey != "" {
		session, err := s.findSessionByLocalStorageKey(localStorageKey)
		if err == nil && session.ExpiresAt.After(now) {
			// 유효한 세션이 있으면 갱신 후 반환
			return s.refreshSession(session)
		}
	}

	// 2. 브라우저 지문으로 기존 세션 찾기 시도
	if browserFingerprint != "" {
		session, err := s.findSessionByFingerprint(browserFingerprint)
		if err == nil && session.ExpiresAt.After(now) {
			// localStorage 키 업데이트 후 반환
			if localStorageKey != "" && session.LocalStorageKey != localStorageKey {
				session.LocalStorageKey = localStorageKey
				s.db.Save(session)
			}
			return s.refreshSession(session)
		}
	}

	// 3. 새로운 세션 생성
	return s.createNewSession(browserFingerprint, localStorageKey)
}

// GetSession 세션 정보 조회
func (s *SessionService) GetSession(sessionID string) (*models.AnonymousSession, error) {
	var session models.AnonymousSession
	result := s.db.Where("session_id = ? AND expires_at > ?", sessionID, time.Now()).First(&session)
	if result.Error != nil {
		return nil, result.Error
	}

	return &session, nil
}

// RefreshSession 세션 만료 시간 갱신
func (s *SessionService) RefreshSession(sessionID string) (*models.AnonymousSession, error) {
	var session models.AnonymousSession
	result := s.db.Where("session_id = ?", sessionID).First(&session)
	if result.Error != nil {
		return nil, result.Error
	}

	return s.refreshSession(&session)
}

// CleanupExpiredSessions 만료된 세션 정리
func (s *SessionService) CleanupExpiredSessions() error {
	result := s.db.Where("expires_at < ?", time.Now()).Delete(&models.AnonymousSession{})
	return result.Error
}

// 내부 메서드들

// findSessionByLocalStorageKey localStorage 키로 세션 찾기
func (s *SessionService) findSessionByLocalStorageKey(key string) (*models.AnonymousSession, error) {
	var session models.AnonymousSession
	result := s.db.Where("local_storage_key = ?", key).First(&session)
	if result.Error != nil {
		return nil, result.Error
	}
	return &session, nil
}

// findSessionByFingerprint 브라우저 지문으로 세션 찾기
func (s *SessionService) findSessionByFingerprint(fingerprint string) (*models.AnonymousSession, error) {
	var session models.AnonymousSession
	result := s.db.Where("browser_fingerprint = ?", fingerprint).First(&session)
	if result.Error != nil {
		return nil, result.Error
	}
	return &session, nil
}

// createNewSession 새로운 세션 생성
func (s *SessionService) createNewSession(browserFingerprint, localStorageKey string) (*models.AnonymousSession, error) {
	sessionID := uuid.New().String()
	nickname := s.generateNickname(sessionID)

	// localStorage 키가 없으면 세션 ID 기반으로 생성
	if localStorageKey == "" {
		localStorageKey = s.generateLocalStorageKey(sessionID)
	}

	session := &models.AnonymousSession{
		SessionID:          sessionID,
		Nickname:           nickname,
		BrowserFingerprint: browserFingerprint,
		LocalStorageKey:    localStorageKey,
		CreatedAt:          time.Now(),
		ExpiresAt:          time.Now().Add(7 * 24 * time.Hour), // 7일 후 만료
	}

	result := s.db.Create(session)
	if result.Error != nil {
		return nil, result.Error
	}

	return session, nil
}

// refreshSession 세션 갱신
func (s *SessionService) refreshSession(session *models.AnonymousSession) (*models.AnonymousSession, error) {
	// 만료 시간을 현재 시간 + 7일로 연장
	session.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)

	result := s.db.Save(session)
	if result.Error != nil {
		return nil, result.Error
	}

	return session, nil
}

// generateNickname 닉네임 자동 생성 (익명_[8자리])
func (s *SessionService) generateNickname(sessionID string) string {
	// 세션 ID에서 8자리 추출 (영숫자만)
	hash := sha256.Sum256([]byte(sessionID))
	hashStr := hex.EncodeToString(hash[:])

	// 영숫자 조합으로 8자리 생성
	var nickname []rune
	for _, char := range hashStr {
		if len(nickname) >= 8 {
			break
		}
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'z') {
			nickname = append(nickname, char)
		}
	}

	return fmt.Sprintf("%s", string(nickname))
}

// generateLocalStorageKey localStorage 키 생성
func (s *SessionService) generateLocalStorageKey(sessionID string) string {
	// 랜덤 바이트와 세션 ID 조합으로 키 생성
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		// 랜덤 바이트 생성 실패 시 시간 기반 대체
		randomBytes = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
	}

	combined := append([]byte(sessionID), randomBytes...)
	hash := sha256.Sum256(combined)

	return hex.EncodeToString(hash[:16]) // 32자리 헥스 문자열
}

// SessionStats 세션 통계 정보
type SessionStats struct {
	TotalSessions   int64 `json:"total_sessions"`
	ActiveSessions  int64 `json:"active_sessions"`
	ExpiredSessions int64 `json:"expired_sessions"`
}

// GetSessionStats 세션 통계 조회
func (s *SessionService) GetSessionStats() (*SessionStats, error) {
	now := time.Now()

	var total, active, expired int64

	// 전체 세션 수
	if err := s.db.Model(&models.AnonymousSession{}).Count(&total).Error; err != nil {
		return nil, err
	}

	// 활성 세션 수
	if err := s.db.Model(&models.AnonymousSession{}).Where("expires_at > ?", now).Count(&active).Error; err != nil {
		return nil, err
	}

	// 만료된 세션 수
	expired = total - active

	return &SessionStats{
		TotalSessions:   total,
		ActiveSessions:  active,
		ExpiredSessions: expired,
	}, nil
}
