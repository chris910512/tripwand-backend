// internal/api/routes/travel.go
package routes

import (
	"tripwand-backend/internal/api/handlers"
	"tripwand-backend/internal/llm"

	"github.com/gofiber/fiber/v2"
)

// SetupTravelRoutes 여행 관련 라우트 설정
func SetupTravelRoutes(api fiber.Router, gemmaClient *llm.GemmaClient) {
	// 여행 핸들러 초기화
	travelHandler := handlers.NewTravelHandler(gemmaClient)

	// 여행 라우트 그룹
	travel := api.Group("/travel")

	// 여행 일정 생성
	travel.Post("/generate", travelHandler.GenerateItinerary)

	// 저장된 여행 계획 목록 조회
	travel.Get("/plans", travelHandler.GetSavedPlans)

	// 특정 여행 계획 상세 조회
	travel.Get("/plans/:id", travelHandler.GetPlanByID)

	// 여행 관련 통계 (선택사항)
	travel.Get("/stats", getTravelStats)
}

// getTravelStats 여행 통계 조회
func getTravelStats(c *fiber.Ctx) error {
	// 간단한 통계 정보 반환
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"total_plans_generated": 1234,
			"popular_destinations":  []string{"부산", "제주도", "서울", "강릉", "경주"},
			"average_duration":      3.2,
		},
	})
}
