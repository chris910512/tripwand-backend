package models

import (
	"time"

	"gorm.io/gorm"
)

// ChatRoom 채팅방 모델
type ChatRoom struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	RoomType     string         `json:"room_type" gorm:"type:varchar(20);not null;check:room_type IN ('public','private')"`
	CountryCode  *string        `json:"country_code" gorm:"type:varchar(20);index"`
	Participant1 *uint          `json:"participant_1_id" gorm:"index"`
	Participant2 *uint          `json:"participant_2_id" gorm:"index"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"deleted_at" gorm:"index"`

	// 관계 설정
	User1    *User        `json:"user1,omitempty" gorm:"foreignKey:Participant1"`
	User2    *User        `json:"user2,omitempty" gorm:"foreignKey:Participant2"`
	Messages []ChatMessage `json:"messages,omitempty" gorm:"foreignKey:RoomID"`
}

// ChatMessage 채팅 메시지 모델 (파티셔닝 테이블)
type ChatMessage struct {
	ID           uint           `json:"id" gorm:"primaryKey;autoIncrement"`  // bigint 타입으로 변경
	RoomID       uint           `json:"room_id" gorm:"not null;index"`
	UserID       *uint          `json:"user_id" gorm:"index"`
	SessionID    *string        `json:"session_id" gorm:"type:varchar(36);index"`
	Content      string         `json:"content" gorm:"type:text;not null"`
	Sender       string         `json:"sender" gorm:"type:varchar(50);not null"`
	MessageType  string         `json:"message_type" gorm:"type:varchar(20);not null;default:'text';check:message_type IN ('text','blocked','system')"`
	ModeratedAt  *time.Time     `json:"moderated_at"`
	IsHarmful    *bool          `json:"is_harmful"`
	CreatedAt    time.Time      `json:"created_at" gorm:"index"`
	DeletedAt    gorm.DeletedAt `json:"deleted_at" gorm:"index"`

	// 관계 설정 (외래키 참조 제거 - 파티셔닝 테이블에서는 문제가 될 수 있음)
	Room ChatRoom `json:"room,omitempty" gorm:"foreignKey:RoomID"`
	User *User    `json:"user,omitempty" gorm:"foreignKey:UserID"`
	// Session 관계는 수동으로 관리 (파티셔닝과 외래키 충돌 방지)
}

// AnonymousSession 익명 세션 모델
type AnonymousSession struct {
	SessionID          string    `json:"session_id" gorm:"type:varchar(36);primaryKey"`
	Nickname           string    `json:"nickname" gorm:"type:varchar(50);not null"`
	BrowserFingerprint string    `json:"browser_fingerprint" gorm:"type:varchar(255)"`
	LocalStorageKey    string    `json:"localstorage_key" gorm:"type:varchar(255)"`
	CreatedAt          time.Time `json:"created_at"`
	ExpiresAt          time.Time `json:"expires_at" gorm:"index"`

	// 관계 설정
	Messages []ChatMessage `json:"messages,omitempty" gorm:"foreignKey:SessionID"`
}

// TableName ChatMessage 테이블명 지정 (파티셔닝 준비)
func (ChatMessage) TableName() string {
	return "chat_messages"
}

// ChatRoomType 채팅방 타입 상수
const (
	RoomTypePublic  = "public"
	RoomTypePrivate = "private"
)

// MessageType 메시지 타입 상수
const (
	MessageTypeText    = "text"
	MessageTypeBlocked = "blocked"
	MessageTypeSystem  = "system"
)

// CountryCode 지원 국가 코드
var SupportedCountries = []string{
	"UK", "US", "France", "Germany", "Spain", "Italy", "Japan",
}

// BeforeCreate 익명 세션 생성 전 처리
func (s *AnonymousSession) BeforeCreate(tx *gorm.DB) error {
	if s.ExpiresAt.IsZero() {
		s.ExpiresAt = s.CreatedAt.AddDate(0, 0, 7) // 7일 후 만료
	}
	return nil
}