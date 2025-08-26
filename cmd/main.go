package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	"tripwand-backend/internal/api/routes"
	"tripwand-backend/internal/database"
	"tripwand-backend/internal/llm"
	ws "tripwand-backend/internal/websocket"
)

var gemmaClient *llm.GemmaClient

func main() {
	// .env 파일 로드
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// 데이터베이스 연결
	log.Println("🔌 Connecting to database...")
	if err := database.Connect(); err != nil {
		log.Printf("⚠️ Failed to connect to database: %v", err)
		log.Println("⚠️ Starting without database connection - some features may not work")
	} else {
		// 데이터베이스 마이그레이션
		log.Println("🔄 Running database migrations...")
		if err := database.Migrate(); err != nil {
			log.Printf("⚠️ Failed to run migrations: %v", err)
		}
	}

	// WebSocket Hub 초기화
	log.Println("🌐 Initializing WebSocket Hub...")
	ws.InitializeHub()

	// Gemma 클라이언트 초기화
	log.Println("🤖 Initializing Gemma client...")
	var err error
	gemmaClient, err = llm.NewGemmaClient()
	if err != nil {
		log.Printf("⚠️ Failed to initialize Gemma client: %v", err)
		log.Println("⚠️ Starting without AI client - some features may not work")
	} else {
		defer func() {
			if err := gemmaClient.Close(); err != nil {
				log.Printf("Failed to close Gemma client: %v", err)
			}
		}()
	}

	// Fiber 앱 초기화
	app := fiber.New(fiber.Config{
		AppName: "TripWand Backend v1.0",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"success": false,
				"message": err.Error(),
				"code":    code,
			})
		},
	})

	// 미들웨어 설정
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} (${latency})\n",
	}))
	app.Use(recover.New())

	// CORS 설정 (개발 및 프로덕션 프론트엔드 지원)
	app.Use(cors.New(cors.Config{
		AllowOrigins:     getEnv("ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"),
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
		AllowCredentials: true,
	}))

	// 헬스체크 엔드포인트
	app.Get("/health", healthCheck)

	// API 라우트 그룹
	api := app.Group("/api/v1")

	// 여행 관련 라우트 설정
	routes.SetupTravelRoutes(api, gemmaClient)

	// 채팅 관련 라우트 설정
	routes.SetupChatRoutes(app)

	// 기존 LLM 라우트 (테스트용으로 유지)
	setupLLMRoutes(api)

	// 포트 설정
	port := getEnv("PORT", "8080")
	log.Printf("🚀 TripWand Backend starting on port %s", port)
	log.Printf("📚 API Documentation:")
	log.Printf("   Health Check: http://localhost:%s/health", port)
	log.Printf("   Travel API: http://localhost:%s/api/v1/travel/generate", port)
	log.Printf("   Plans API: http://localhost:%s/api/v1/travel/plans", port)
	log.Printf("   Chat WebSocket: ws://localhost:%s/ws/chat", port)
	log.Printf("   Chat Rooms: http://localhost:%s/api/v1/chat/rooms", port)
	log.Printf("   Chat Stats: http://localhost:%s/api/v1/chat/stats", port)

	if err := app.Listen(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}


// healthCheck 헬스체크 핸들러
func healthCheck(c *fiber.Ctx) error {
	// 데이터베이스 연결 상태 확인
	sqlDB, err := database.DB.DB()
	dbStatus := "healthy"
	if err != nil || sqlDB.Ping() != nil {
		dbStatus = "unhealthy"
	}

	// Gemma 클라이언트 상태 확인
	gemmaStatus := "healthy"
	if gemmaClient == nil {
		gemmaStatus = "unhealthy"
	}

	// WebSocket Hub 상태 확인
	webSocketStatus := "healthy"
	stats := ws.GlobalHub.GetStats()
	if ws.GlobalHub == nil {
		webSocketStatus = "unhealthy"
	}

	return c.JSON(fiber.Map{
		"status":    "healthy",
		"service":   "tripwand-backend",
		"version":   "1.0.0",
		"database":  dbStatus,
		"gemma":     gemmaStatus,
		"websocket": webSocketStatus,
		"chat_stats": fiber.Map{
			"total_clients": stats.TotalClients,
			"active_rooms":  len(stats.RoomClients),
		},
		"features": []string{
			"여행 일정 AI 생성",
			"실시간 채팅 (7개국)",
			"Google AI Studio Gemma 3",
			"PostgreSQL + pgvector",
			"여행 계획 저장/조회",
		},
		"endpoints": []string{
			"POST /api/v1/travel/generate - 여행 일정 생성",
			"GET /api/v1/travel/plans - 저장된 계획 목록",
			"GET /api/v1/travel/plans/{id} - 계획 상세 조회",
			"WebSocket /ws/chat - 실시간 채팅",
			"GET /api/v1/chat/rooms - 채팅방 목록",
			"GET /api/v1/chat/stats - 채팅 통계",
		},
	})
}

// setupLLMRoutes 기존 LLM 테스트 라우트 (유지)
func setupLLMRoutes(api fiber.Router) {
	llmGroup := api.Group("/llm")

	// Gemma 채팅 엔드포인트 (테스트용)
	llmGroup.Post("/chat", handleGemmaChat)

	// Gemma 텍스트 생성 엔드포인트 (테스트용)
	llmGroup.Post("/generate", handleGemmaGenerate)
}

// handleGemmaChat 기존 Gemma 채팅 핸들러
func handleGemmaChat(c *fiber.Ctx) error {
	var req llm.ChatRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	if req.Message == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Message is required",
		})
	}

	response, err := gemmaClient.Chat(req)
	if err != nil {
		log.Printf("Gemma chat error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate response",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// handleGemmaGenerate 기존 Gemma 텍스트 생성 핸들러
func handleGemmaGenerate(c *fiber.Ctx) error {
	var req llm.GenerateRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	if req.Prompt == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Prompt is required",
		})
	}

	// 기본값 설정
	if req.MaxTokens == 0 {
		req.MaxTokens = 1000
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	response, err := gemmaClient.Generate(req)
	if err != nil {
		log.Printf("Gemma generate error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate text",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// getEnv 환경 변수 헬퍼 함수
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
