package websocket

import (
	"encoding/json"
	"fmt"
	"time"
)

// MessageType 메시지 타입 정의
type MessageType string

const (
	// 클라이언트 → 서버
	MessageTypeJoinRoom    MessageType = "join_room"    // 채팅방 입장
	MessageTypeChatMessage MessageType = "chat_message" // 채팅 메시지 전송
	MessageTypePong        MessageType = "pong"         // Heartbeat 응답

	// 서버 → 클라이언트
	MessageTypeRoomJoined  MessageType = "room_joined"   // 채팅방 입장 완료
	MessageTypeNewMessage  MessageType = "new_message"   // 새 메시지 브로드캐스트
	MessageTypeBlocked     MessageType = "message_blocked" // 메시지 차단 알림
	MessageTypePing        MessageType = "ping"          // Heartbeat
	MessageTypeError       MessageType = "error"         // 에러 메시지
	MessageTypeUserCount   MessageType = "user_count"    // 사용자 수 업데이트
	MessageTypeUserJoined  MessageType = "user_joined"   // 사용자 입장 알림
	MessageTypeUserLeft    MessageType = "user_left"     // 사용자 퇴장 알림
)

// Message WebSocket 메시지 구조
type Message struct {
	Type      MessageType            `json:"type"`                // 메시지 타입
	Room      string                 `json:"room,omitempty"`      // 채팅방 이름
	Content   string                 `json:"content,omitempty"`   // 메시지 내용
	UserID    *uint                  `json:"user_id,omitempty"`   // 사용자 ID (로그인 사용자)
	SessionID *string                `json:"session_id,omitempty"` // 세션 ID (익명 사용자)
	Nickname  string                 `json:"nickname,omitempty"`  // 닉네임
	MessageID string                 `json:"message_id,omitempty"` // 메시지 ID (DB 저장 후)
	Timestamp time.Time              `json:"timestamp"`           // 타임스탬프
	Data      map[string]interface{} `json:"data,omitempty"`      // 추가 데이터
}

// ChatMessageData 채팅 메시지 데이터
type ChatMessageData struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Sender    string    `json:"sender"`    // 닉네임
	UserID    *uint     `json:"user_id,omitempty"`
	SessionID *string   `json:"session_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	IsBlocked bool      `json:"is_blocked,omitempty"`
}

// RoomJoinedData 방 입장 완료 데이터
type RoomJoinedData struct {
	Room      string `json:"room"`
	UserCount int    `json:"user_count"`
}

// MessageBlockedData 메시지 차단 데이터
type MessageBlockedData struct {
	MessageID string `json:"message_id"`
	Reason    string `json:"reason"`
}

// ErrorData 에러 데이터
type ErrorData struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
}

// UserEventData 사용자 이벤트 데이터 (입장/퇴장)
type UserEventData struct {
	UserID    *uint   `json:"user_id,omitempty"`
	SessionID *string `json:"session_id,omitempty"`
	Nickname  string  `json:"nickname"`
	UserCount int     `json:"user_count"`
}

// ToJSON 메시지를 JSON 바이트로 변환
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON JSON 바이트에서 메시지로 변환
func (m *Message) FromJSON(data []byte) error {
	return json.Unmarshal(data, m)
}

// NewJoinRoomMessage 방 입장 메시지 생성
func NewJoinRoomMessage(room string, sessionID *string, userID *uint, nickname string) *Message {
	data := make(map[string]interface{})
	if sessionID != nil {
		data["session_id"] = *sessionID
	}
	if userID != nil {
		data["user_id"] = *userID
	}
	if nickname != "" {
		data["nickname"] = nickname
	}

	return &Message{
		Type:      MessageTypeJoinRoom,
		Room:      room,
		Timestamp: time.Now(),
		Data:      data,
	}
}

// NewChatMessage 채팅 메시지 생성
func NewChatMessage(room, content, nickname string, userID *uint, sessionID *string) *Message {
	return &Message{
		Type:      MessageTypeChatMessage,
		Room:      room,
		Content:   content,
		UserID:    userID,
		SessionID: sessionID,
		Nickname:  nickname,
		Timestamp: time.Now(),
	}
}

// NewRoomJoinedMessage 방 입장 완료 메시지 생성
func NewRoomJoinedMessage(room string, userCount int) *Message {
	return &Message{
		Type: MessageTypeRoomJoined,
		Room: room,
		Data: map[string]interface{}{
			"user_count": userCount,
		},
		Timestamp: time.Now(),
	}
}

// NewNewMessageBroadcast 새 메시지 브로드캐스트 생성
func NewNewMessageBroadcast(room, content, nickname, messageID string, userID *uint, sessionID *string) *Message {
	data := map[string]interface{}{
		"id":         messageID,
		"content":    content,
		"sender":     nickname,
		"created_at": time.Now(),
	}

	if userID != nil {
		data["user_id"] = *userID
	}
	if sessionID != nil {
		data["session_id"] = *sessionID
	}

	return &Message{
		Type:      MessageTypeNewMessage,
		Room:      room,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewMessageBlockedBroadcast 메시지 차단 브로드캐스트 생성
func NewMessageBlockedBroadcast(messageID, reason string) *Message {
	return &Message{
		Type: MessageTypeBlocked,
		Data: map[string]interface{}{
			"message_id": messageID,
			"reason":     reason,
		},
		Timestamp: time.Now(),
	}
}

// NewPingMessage Ping 메시지 생성
func NewPingMessage() *Message {
	return &Message{
		Type:      MessageTypePing,
		Timestamp: time.Now(),
	}
}

// NewPongMessage Pong 메시지 생성
func NewPongMessage() *Message {
	return &Message{
		Type:      MessageTypePong,
		Timestamp: time.Now(),
	}
}

// NewErrorMessage 에러 메시지 생성
func NewErrorMessage(errorMsg string, code ...int) *Message {
	data := map[string]interface{}{
		"message": errorMsg,
	}

	if len(code) > 0 {
		data["code"] = code[0]
	}

	return &Message{
		Type:      MessageTypeError,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewUserJoinedMessage 사용자 입장 알림 메시지 생성
func NewUserJoinedMessage(room, nickname string, userID *uint, sessionID *string, userCount int) *Message {
	data := map[string]interface{}{
		"nickname":   nickname,
		"user_count": userCount,
	}

	if userID != nil {
		data["user_id"] = *userID
	}
	if sessionID != nil {
		data["session_id"] = *sessionID
	}

	return &Message{
		Type:      MessageTypeUserJoined,
		Room:      room,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewUserLeftMessage 사용자 퇴장 알림 메시지 생성
func NewUserLeftMessage(room, nickname string, userID *uint, sessionID *string, userCount int) *Message {
	data := map[string]interface{}{
		"nickname":   nickname,
		"user_count": userCount,
	}

	if userID != nil {
		data["user_id"] = *userID
	}
	if sessionID != nil {
		data["session_id"] = *sessionID
	}

	return &Message{
		Type:      MessageTypeUserLeft,
		Room:      room,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewUserCountMessage 사용자 수 업데이트 메시지 생성
func NewUserCountMessage(room string, userCount int) *Message {
	return &Message{
		Type: MessageTypeUserCount,
		Room: room,
		Data: map[string]interface{}{
			"user_count": userCount,
		},
		Timestamp: time.Now(),
	}
}

// IsClientMessage 클라이언트에서 서버로 보내는 메시지인지 확인
func (m *Message) IsClientMessage() bool {
	switch m.Type {
	case MessageTypeJoinRoom, MessageTypeChatMessage, MessageTypePong:
		return true
	default:
		return false
	}
}

// IsServerMessage 서버에서 클라이언트로 보내는 메시지인지 확인
func (m *Message) IsServerMessage() bool {
	switch m.Type {
	case MessageTypeRoomJoined, MessageTypeNewMessage, MessageTypeBlocked, 
		 MessageTypePing, MessageTypeError, MessageTypeUserCount, 
		 MessageTypeUserJoined, MessageTypeUserLeft:
		return true
	default:
		return false
	}
}

// Validate 메시지 유효성 검사
func (m *Message) Validate() error {
	if m.Type == "" {
		return fmt.Errorf("message type is required")
	}

	switch m.Type {
	case MessageTypeJoinRoom:
		if m.Room == "" {
			return fmt.Errorf("room is required for join_room message")
		}
	
	case MessageTypeChatMessage:
		if m.Room == "" {
			return fmt.Errorf("room is required for chat_message")
		}
		if m.Content == "" {
			return fmt.Errorf("content is required for chat_message")
		}
		if len(m.Content) > 280 {
			return fmt.Errorf("message content too long (max 280 characters)")
		}
	}

	return nil
}

// SanitizeContent 메시지 내용 정제 (HTML 태그 제거 등)
func (m *Message) SanitizeContent() {
	// 기본적인 HTML 태그 제거 (보안상 중요)
	// 실제 프로덕션에서는 더 강력한 sanitization이 필요
	if m.Content != "" {
		// 여기에 HTML sanitization 로직 추가
		// 예: html.EscapeString(m.Content)
	}
}