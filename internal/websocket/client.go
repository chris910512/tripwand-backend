package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/websocket/v2"

	"tripwand-backend/internal/services"
)

// Client WebSocket 클라이언트 연결을 나타냄
type Client struct {
	// 고유 ID
	ID string

	// WebSocket 연결
	conn *websocket.Conn

	// 허브 참조
	hub *Hub

	// 메시지 전송 채널
	send chan []byte

	// 클라이언트가 속한 채팅방
	Room string

	// 사용자 ID (로그인한 경우)
	UserID *uint

	// 세션 ID (익명 사용자인 경우)
	SessionID *string

	// 닉네임
	Nickname string

	// 연결 시간
	ConnectedAt time.Time

	// 마지막 ping 시간
	LastPing time.Time

	// 클라이언트가 활성 상태인지
	IsActive bool
}

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// NewClient 새로운 클라이언트 생성
func NewClient(conn *websocket.Conn, hub *Hub, clientID string) *Client {
	now := time.Now()
	return &Client{
		ID:          clientID,
		conn:        conn,
		hub:         hub,
		send:        make(chan []byte, 256),
		ConnectedAt: now,
		LastPing:    now,
		IsActive:    true,
	}
}

// ReadPump 클라이언트로부터 메시지를 읽는 고루틴
func (c *Client) ReadPump() {
	defer func() {
		c.hub.UnregisterClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Printf("Failed to set read deadline: %v", err)
	}
	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			log.Printf("Failed to set read deadline in pong handler: %v", err)
		}
		c.LastPing = time.Now()
		return nil
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		log.Printf("[DEBUG] Raw message received from client %s: %s", c.ID, string(messageBytes))

		// 메시지 처리
		if err := c.handleMessage(messageBytes); err != nil {
			log.Printf("Error handling message from client %s: %v", c.ID, err)
		}

		c.LastPing = time.Now()
	}
}

// WritePump 클라이언트에게 메시지를 쓰는 고루틴
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Printf("Failed to set write deadline: %v", err)
				return
			}
			if !ok {
				// Hub가 채널을 닫았음
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Printf("Failed to write close message: %v", err)
				}
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				log.Printf("Failed to write message: %v", err)
				return
			}

			// 큐에 있는 추가 메시지들도 함께 전송
			n := len(c.send)
			for i := 0; i < n; i++ {
				if _, err := w.Write([]byte{'\n'}); err != nil {
					log.Printf("Failed to write newline: %v", err)
					return
				}
				if _, err := w.Write(<-c.send); err != nil {
					log.Printf("Failed to write queued message: %v", err)
					return
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Printf("Failed to set write deadline for ping: %v", err)
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 클라이언트로부터 받은 메시지 처리
func (c *Client) handleMessage(messageBytes []byte) error {
	var msg Message
	if err := json.Unmarshal(messageBytes, &msg); err != nil {
		return err
	}

	// 메시지 타입별 처리
	switch msg.Type {
	case MessageTypeJoinRoom:
		return c.handleJoinRoom(&msg)
	
	case MessageTypeChatMessage:
		return c.handleChatMessage(&msg)
	
	case MessageTypePong:
		return c.handlePong(&msg)
	
	default:
		log.Printf("Unknown message type: %s from client %s", msg.Type, c.ID)
	}

	return nil
}

// handleJoinRoom 방 입장 처리
func (c *Client) handleJoinRoom(msg *Message) error {
	log.Printf("[DEBUG] handleJoinRoom called for client %s", c.ID)
	log.Printf("[DEBUG] Message data: %+v", msg)
	
	// 기존 방에서 나가기
	if c.Room != "" {
		c.leaveRoom()
	}

	// 새 방 입장
	c.Room = msg.Room
	log.Printf("[DEBUG] Room set to: %s", c.Room)
	
	// 세션 ID 설정 (익명 사용자인 경우)
	if sessionID, ok := msg.Data["session_id"].(string); ok {
		c.SessionID = &sessionID
	}

	// 사용자 ID 설정 (로그인 사용자인 경우) 
	if userID, ok := msg.Data["user_id"].(float64); ok {
		uid := uint(userID)
		c.UserID = &uid
	}

	// 닉네임 설정
	if nickname, ok := msg.Data["nickname"].(string); ok {
		c.Nickname = nickname
	}

	log.Printf("Client %s joining room %s", c.ID, c.Room)
	
	// 허브에 다시 등록 (방 정보 업데이트)
	c.hub.RegisterClient(c)

	return nil
}

// handleChatMessage 채팅 메시지 처리
func (c *Client) handleChatMessage(msg *Message) error {
	log.Printf("[DEBUG-CHAT] handleChatMessage called for client %s", c.ID)
	log.Printf("[DEBUG-CHAT] Client info - Room: %s, UserID: %v, SessionID: %v, Nickname: %s", c.Room, c.UserID, c.SessionID, c.Nickname)
	log.Printf("[DEBUG-CHAT] Message content: %s", msg.Content)
	
	if c.Room == "" {
		log.Printf("[DEBUG-CHAT] ERROR: Client not in any room")
		return c.sendError("You must join a room first")
	}

	// 메시지 검증
	if msg.Content == "" {
		log.Printf("[DEBUG-CHAT] ERROR: Empty message content")
		return c.sendError("Message content cannot be empty")
	}

	if len(msg.Content) > 280 {
		log.Printf("[DEBUG-CHAT] ERROR: Message too long (%d chars)", len(msg.Content))
		return c.sendError("Message too long (max 280 characters)")
	}

	// 메시지 서비스 초기화
	messageService := services.NewMessageService()
	log.Printf("[DEBUG-CHAT] MessageService initialized")
	
	// 채팅방 정보 조회
	log.Printf("[DEBUG-CHAT] Getting room info for country code: %s", c.Room)
	room, err := messageService.GetRoomByCountryCode(c.Room)
	if err != nil {
		log.Printf("[DEBUG-CHAT] ERROR: Failed to get room for country %s: %v", c.Room, err)
		return c.sendError("Invalid room")
	}
	countryCode := ""
	if room.CountryCode != nil {
		countryCode = *room.CountryCode
	}
	log.Printf("[DEBUG-CHAT] Room found - ID: %d, Country: %s", room.ID, countryCode)

	// 데이터베이스에 메시지 저장
	log.Printf("[DEBUG-CHAT] Attempting to save message to DB...")
	log.Printf("[DEBUG-CHAT] Save params - RoomID: %d, Content: %s, UserID: %v, SessionID: %v, Nickname: %s", room.ID, msg.Content, c.UserID, c.SessionID, c.Nickname)
	
	savedMessage, err := messageService.SaveChatMessage(room.ID, msg.Content, c.UserID, c.SessionID, c.Nickname)
	if err != nil {
		log.Printf("[DEBUG-CHAT] ERROR: Failed to save message: %v", err)
		// 저장 실패해도 실시간 전송은 계속 진행
	} else {
		log.Printf("[DEBUG-CHAT] SUCCESS: Message saved to DB with ID: %d", savedMessage.ID)
	}

	// 메시지 ID 설정 (저장된 경우)
	messageID := ""
	if savedMessage != nil {
		messageID = fmt.Sprintf("%d", savedMessage.ID)  // uint를 string으로 변환
		log.Printf("[DEBUG-CHAT] Using saved message ID: %s", messageID)
	} else {
		log.Printf("[DEBUG-CHAT] WARNING: No saved message ID (savedMessage is nil)")
	}

	// 브로드캐스트용 메시지 생성
	broadcastMsg := NewNewMessageBroadcast(
		c.Room,
		msg.Content,
		c.Nickname,
		messageID,
		c.UserID,
		c.SessionID,
	)

	// 방의 모든 클라이언트에게 브로드캐스트
	messageBytes, err := broadcastMsg.ToJSON()
	if err != nil {
		log.Printf("[DEBUG-CHAT] ERROR: Failed to convert broadcast message to JSON: %v", err)
		return err
	}

	log.Printf("[DEBUG-CHAT] Broadcasting message to room %s: %s", c.Room, string(messageBytes))
	c.hub.BroadcastToRoom(c.Room, messageBytes)
	log.Printf("[DEBUG-CHAT] Broadcast completed")
	
	// TODO: AI 검열 처리 (비동기)

	log.Printf("Chat message from %s in room %s: %s", c.ID, c.Room, msg.Content)

	return nil
}

// handlePong pong 메시지 처리
func (c *Client) handlePong(msg *Message) error {
	c.LastPing = time.Now()
	log.Printf("Pong received from client %s", c.ID)
	return nil
}

// leaveRoom 현재 방에서 나가기
func (c *Client) leaveRoom() {
	if c.Room != "" {
		log.Printf("Client %s leaving room %s", c.ID, c.Room)
		c.Room = ""
	}
}

// sendError 클라이언트에게 에러 메시지 전송
func (c *Client) sendError(errorMsg string) error {
	errorMessage := &Message{
		Type:      MessageTypeError,
		Content:   errorMsg,
		Timestamp: time.Now(),
	}

	return c.SendMessage(errorMessage)
}

// SendMessage 클라이언트에게 메시지 전송
func (c *Client) SendMessage(msg *Message) error {
	messageBytes, err := msg.ToJSON()
	if err != nil {
		return err
	}

	select {
	case c.send <- messageBytes:
		return nil
	default:
		// 채널이 가득 찬 경우
		return c.sendError("Message queue full")
	}
}

// GetInfo 클라이언트 정보 반환
func (c *Client) GetInfo() *ClientInfo {
	return &ClientInfo{
		ID:          c.ID,
		Room:        c.Room,
		UserID:      c.UserID,
		SessionID:   c.SessionID,
		ConnectedAt: c.ConnectedAt,
		LastPing:    c.LastPing,
	}
}

// Close 클라이언트 연결 종료
func (c *Client) Close() {
	c.IsActive = false
	if c.send != nil {
		close(c.send)
	}
	if c.conn != nil {
		c.conn.Close()
	}
}