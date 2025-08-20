package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	// .env 파일 로드
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Fiber 앱 초기화
	app := fiber.New(fiber.Config{
		AppName: "Go WebService Server v1.0",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"success": false,
				"message": err.Error(),
			})
		},
	})

	// 미들웨어 설정
	app.Use(logger.New())
	app.Use(recover.New())

	// CORS 설정 (Vercel 프론트엔드 지원)
	app.Use(cors.New(cors.Config{
		AllowOrigins:     getEnv("ALLOWED_ORIGINS", "http://localhost:3000"),
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
		AllowCredentials: true,
	}))

	// 헬스체크 엔드포인트
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"service": "go-webservice-server",
			"version": "1.0.0",
		})
	})

	// API 라우트 그룹
	api := app.Group("/api/v1")

	// LLM(Gemma) 라우트
	setupLLMRoutes(api)

	// 포트 설정
	port := getEnv("PORT", "8080")
	log.Printf("🚀 Server starting on port %s", port)

	if err := app.Listen(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// LLM 라우트 설정
func setupLLMRoutes(api fiber.Router) {
	llm := api.Group("/llm")

	// Gemma 채팅 엔드포인트
	llm.Post("/chat", handleGemmaChat)

	// Gemma 텍스트 생성 엔드포인트
	llm.Post("/generate", handleGemmaGenerate)
}

// Gemma 채팅 핸들러 (임시 구현)
func handleGemmaChat(c *fiber.Ctx) error {
	var req struct {
		Message string `json:"message" validate:"required"`
		History []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"history,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Message == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Message is required",
		})
	}

	// TODO: 실제 Gemma API 호출 구현
	response := "Hello! This is a placeholder response from Gemma. Your message: " + req.Message

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"response": response,
			"model":    "gemma-3-27b",
		},
	})
}

// Gemma 텍스트 생성 핸들러 (임시 구현)
func handleGemmaGenerate(c *fiber.Ctx) error {
	var req struct {
		Prompt      string  `json:"prompt" validate:"required"`
		MaxTokens   int     `json:"max_tokens,omitempty"`
		Temperature float32 `json:"temperature,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
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

	// TODO: 실제 Gemma API 호출 구현
	response := "Generated text based on prompt: " + req.Prompt

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"generated_text": response,
			"model":          "gemma-3-27b",
			"prompt":         req.Prompt,
			"settings": fiber.Map{
				"max_tokens":  req.MaxTokens,
				"temperature": req.Temperature,
			},
		},
	})
}

// 환경 변수 헬퍼 함수
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
