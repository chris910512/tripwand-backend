package handlers

import (
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"

	"tripwand-backend/internal/services"
	ws "tripwand-backend/internal/websocket"
)

// ChatHandler 채팅 관련 핸들러
type ChatHandler struct {
	sessionService *services.SessionService
	messageService *services.MessageService
}

// NewChatHandler 새로운 채팅 핸들러 생성
func NewChatHandler() *ChatHandler {
	return &ChatHandler{
		sessionService: services.NewSessionService(),
		messageService: services.NewMessageService(),
	}
}

// WebSocketUpgrade WebSocket 업그레이드 핸들러
func (h *ChatHandler) WebSocketUpgrade(c *fiber.Ctx) error {
	// WebSocket 업그레이드 요청인지 확인
	if websocket.IsWebSocketUpgrade(c) {
		c.Locals("allowed", true)
		return c.Next()
	}
	return fiber.ErrUpgradeRequired
}

// HandleWebSocket WebSocket 연결 핸들러
func (h *ChatHandler) HandleWebSocket(c *websocket.Conn) {
	// 클라이언트 ID 생성
	clientID := uuid.New().String()

	// 새 클라이언트 생성
	client := ws.NewClient(c, ws.GlobalHub, clientID)

	log.Printf("New WebSocket connection: %s", clientID)

	// 클라이언트를 허브에 등록
	ws.GlobalHub.RegisterClient(client)

	// 읽기/쓰기 고루틴 시작
	go client.WritePump()
	go client.ReadPump()
}

// GetRooms 채팅방 목록 조회
func (h *ChatHandler) GetRooms(c *fiber.Ctx) error {
	// 지원하는 7개 국가 채팅방 정보
	rooms := []map[string]interface{}{
		{
			"id":           "UK",
			"name":         "United Kingdom",
			"country_code": "UK",
			"type":         "public",
			"user_count":   ws.GlobalHub.GetRoomClientCount("UK"),
		},
		{
			"id":           "US",
			"name":         "United States",
			"country_code": "US", 
			"type":         "public",
			"user_count":   ws.GlobalHub.GetRoomClientCount("US"),
		},
		{
			"id":           "France",
			"name":         "France",
			"country_code": "France",
			"type":         "public",
			"user_count":   ws.GlobalHub.GetRoomClientCount("France"),
		},
		{
			"id":           "Germany",
			"name":         "Germany",
			"country_code": "Germany",
			"type":         "public",
			"user_count":   ws.GlobalHub.GetRoomClientCount("Germany"),
		},
		{
			"id":           "Spain",
			"name":         "Spain",
			"country_code": "Spain",
			"type":         "public",
			"user_count":   ws.GlobalHub.GetRoomClientCount("Spain"),
		},
		{
			"id":           "Italy",
			"name":         "Italy",
			"country_code": "Italy",
			"type":         "public",
			"user_count":   ws.GlobalHub.GetRoomClientCount("Italy"),
		},
		{
			"id":           "Japan",
			"name":         "Japan",
			"country_code": "Japan",
			"type":         "public",
			"user_count":   ws.GlobalHub.GetRoomClientCount("Japan"),
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"rooms":   rooms,
	})
}

// GetRoomMessages 채팅방 메시지 이력 조회 (페이징)
func (h *ChatHandler) GetRoomMessages(c *fiber.Ctx) error {
	room := c.Params("room")
	if room == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Room parameter is required",
		})
	}

	// 페이징 파라미터
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	// 데이터베이스에서 메시지 조회
	messages, total, err := h.messageService.GetRoomMessagesByCountryCode(room, page, limit)
	if err != nil {
		log.Printf("Failed to get messages for room %s: %v", room, err)
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"message": "Room not found or failed to fetch messages",
		})
	}

	// 응답 형식으로 변환
	messageResponses := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		messageResponses[i] = map[string]interface{}{
			"id":         msg.ID,
			"content":    msg.Content,
			"sender":     msg.Sender,
			"user_id":    msg.UserID,
			"session_id": msg.SessionID,
			"message_type": msg.MessageType,
			"created_at": msg.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"is_blocked": msg.MessageType == "blocked",
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"room":    room,
		"page":    page,
		"limit":   limit,
		"total":   total,
		"messages": messageResponses,
	})
}

// GetStats WebSocket 허브 통계 조회
func (h *ChatHandler) GetStats(c *fiber.Ctx) error {
	stats := ws.GlobalHub.GetStats()
	
	// 세션 통계도 포함
	sessionStats, err := h.sessionService.GetSessionStats()
	if err != nil {
		log.Printf("Failed to get session stats: %v", err)
		sessionStats = &services.SessionStats{}
	}
	
	// 메시지 통계도 포함
	messageStats, err := h.messageService.GetMessageStats()
	if err != nil {
		log.Printf("Failed to get message stats: %v", err)
		messageStats = &services.MessageStats{}
	}
	
	return c.JSON(fiber.Map{
		"success":       true,
		"websocket":     stats,
		"sessions":      sessionStats,
		"messages":      messageStats,
	})
}

// CreateSession 익명 세션 생성/복구
func (h *ChatHandler) CreateSession(c *fiber.Ctx) error {
	type SessionRequest struct {
		BrowserFingerprint string `json:"browser_fingerprint"`
		LocalStorageKey    string `json:"localstorage_key"`
	}

	var req SessionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request format",
		})
	}

	// 세션 생성/복구 시도
	session, err := h.sessionService.CreateOrRecoverSession(req.BrowserFingerprint, req.LocalStorageKey)
	if err != nil {
		log.Printf("Failed to create/recover session: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create session",
		})
	}

	return c.JSON(fiber.Map{
		"success":           true,
		"session_id":        session.SessionID,
		"nickname":          session.Nickname,
		"localstorage_key":  session.LocalStorageKey,
		"expires_at":        session.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		"created_at":        session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"is_recovered":      req.LocalStorageKey != "" && session.LocalStorageKey == req.LocalStorageKey,
	})
}

// GetSession 세션 정보 조회
func (h *ChatHandler) GetSession(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Session ID is required",
		})
	}

	// 세션 정보 조회
	session, err := h.sessionService.GetSession(sessionID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"message": "Session not found or expired",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"session_id":       session.SessionID,
		"nickname":         session.Nickname,
		"localstorage_key": session.LocalStorageKey,
		"expires_at":       session.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		"created_at":       session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"is_active":        time.Now().Before(session.ExpiresAt),
	})
}

// RefreshSession 세션 갱신 (만료시간 연장)
func (h *ChatHandler) RefreshSession(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if sessionID == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Session ID is required",
		})
	}

	// 세션 갱신
	session, err := h.sessionService.RefreshSession(sessionID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"message": "Session not found",
		})
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"session_id": session.SessionID,
		"expires_at": session.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		"message":    "Session refreshed successfully",
	})
}

// SendTestMessage 테스트용 메시지 전송 (개발/테스트용)
func (h *ChatHandler) SendTestMessage(c *fiber.Ctx) error {
	type TestMessageRequest struct {
		Room    string `json:"room"`
		Content string `json:"content"`
		Sender  string `json:"sender"`
	}

	var req TestMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request format",
		})
	}

	if req.Room == "" || req.Content == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Room and content are required",
		})
	}

	// 테스트 메시지 생성
	testMsg := ws.NewNewMessageBroadcast(
		req.Room,
		req.Content,
		req.Sender,
		"test-"+uuid.New().String(),
		nil, // userID
		nil, // sessionID
	)

	// 방에 브로드캐스트
	messageBytes, err := testMsg.ToJSON()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create message",
		})
	}

	ws.GlobalHub.BroadcastToRoom(req.Room, messageBytes)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Test message sent",
		"data":    testMsg,
	})
}

// BroadcastToRoom 특정 방에 메시지 브로드캐스트 (관리용)
func (h *ChatHandler) BroadcastToRoom(c *fiber.Ctx) error {
	type BroadcastRequest struct {
		Room    string                 `json:"room"`
		Type    string                 `json:"type"`
		Content string                 `json:"content"`
		Data    map[string]interface{} `json:"data"`
	}

	var req BroadcastRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request format",
		})
	}

	if req.Room == "" || req.Type == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Room and type are required",
		})
	}

	// 브로드캐스트 메시지 생성
	msg := &ws.Message{
		Type:      ws.MessageType(req.Type),
		Room:      req.Room,
		Content:   req.Content,
		Data:      req.Data,
		Timestamp: time.Now(),
	}

	messageBytes, err := msg.ToJSON()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create message",
		})
	}

	ws.GlobalHub.BroadcastToRoom(req.Room, messageBytes)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Message broadcasted to room",
		"room":    req.Room,
		"type":    req.Type,
	})
}