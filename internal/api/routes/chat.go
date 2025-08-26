package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"

	"tripwand-backend/internal/api/handlers"
)

// SetupChatRoutes 채팅 관련 라우트 설정
func SetupChatRoutes(app *fiber.App) {
	chatHandler := handlers.NewChatHandler()

	// WebSocket 엔드포인트
	// WebSocket 업그레이드 미들웨어 적용
	app.Use("/ws", chatHandler.WebSocketUpgrade)
	app.Get("/ws/chat", websocket.New(chatHandler.HandleWebSocket, websocket.Config{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		Origins: []string{"*"}, // CORS 설정 - 운영 환경에서는 더 엄격하게 설정
		EnableCompression: true,
	}))

	// REST API 엔드포인트 그룹
	api := app.Group("/api/v1/chat")

	// 채팅방 관련
	api.Get("/rooms", chatHandler.GetRooms)                           // 채팅방 목록
	api.Get("/rooms/:room/messages", chatHandler.GetRoomMessages)     // 메시지 이력

	// 세션 관련 (익명 사용자)
	api.Post("/sessions", chatHandler.CreateSession)                  // 세션 생성/복구
	api.Get("/sessions/:sessionId", chatHandler.GetSession)           // 세션 정보 조회

	// 통계 및 관리
	api.Get("/stats", chatHandler.GetStats)                          // WebSocket 통계

	// 개발/테스트용 엔드포인트
	api.Post("/test/message", chatHandler.SendTestMessage)           // 테스트 메시지 전송
	api.Post("/broadcast/:room", chatHandler.BroadcastToRoom)        // 방 브로드캐스트
}