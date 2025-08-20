// internal/api/handlers/travel.go
package handlers

import (
	"encoding/json"
	"log"
	"strings"

	"tripwand-backend/internal/database"
	"tripwand-backend/internal/llm"
	"tripwand-backend/internal/models"

	"github.com/gofiber/fiber/v2"
)

// TravelHandler 여행 관련 핸들러
type TravelHandler struct {
	gemmaClient *llm.GemmaClient
}

// NewTravelHandler 새로운 여행 핸들러 생성
func NewTravelHandler(gemmaClient *llm.GemmaClient) *TravelHandler {
	return &TravelHandler{
		gemmaClient: gemmaClient,
	}
}

// GenerateItinerary 여행 일정 생성
// @Summary 여행 일정 생성
// @Description 목적지, 기간 등의 정보를 바탕으로 AI가 여행 일정을 생성합니다
// @Tags travel
// @Accept json
// @Produce json
// @Param request body models.TravelRequest true "여행 요청 정보"
// @Success 200 {object} models.TravelResponse "생성된 여행 일정"
// @Failure 400 {object} map[string]interface{} "잘못된 요청"
// @Failure 500 {object} map[string]interface{} "서버 오류"
// @Router /api/v1/travel/generate [post]
func (h *TravelHandler) GenerateItinerary(c *fiber.Ctx) error {
	var req models.TravelRequest

	// 요청 파싱
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "잘못된 요청 형식입니다",
			"error":   err.Error(),
		})
	}

	// 필수값 검증
	if req.Destination == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "목적지는 필수입니다",
		})
	}

	if req.Duration <= 0 || req.Duration > 30 {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "여행 기간은 1일 이상 30일 이하여야 합니다",
		})
	}

	// Gemma 프롬프트 생성
	prompt := req.ToGemmaPrompt()

	log.Printf("Generated prompt for destination: %s, duration: %d days", req.Destination, req.Duration)

	// Gemma API 호출
	gemmaReq := llm.GenerateRequest{
		Prompt:      prompt,
		Temperature: 0.7,
		MaxTokens:   2000, // 긴 응답을 위해 토큰 수 증가
	}

	gemmaResp, err := h.gemmaClient.Generate(gemmaReq)
	if err != nil {
		log.Printf("Gemma API error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "AI 서비스 호출 중 오류가 발생했습니다",
			"error":   err.Error(),
		})
	}

	// JSON 응답 파싱
	var travelResponse models.TravelResponse

	// Gemma 응답에서 JSON 부분만 추출
	jsonText := extractJSON(gemmaResp.GeneratedText)

	if err := json.Unmarshal([]byte(jsonText), &travelResponse); err != nil {
		log.Printf("JSON parse error: %v, raw response: %s", err, gemmaResp.GeneratedText)
		return c.Status(500).JSON(fiber.Map{
			"success":      false,
			"message":      "AI 응답 처리 중 오류가 발생했습니다",
			"error":        "응답 형식이 올바르지 않습니다",
			"raw_response": gemmaResp.GeneratedText,
		})
	}

	// 일정 검증 (기간과 생성된 일수가 맞는지 확인)
	if len(travelResponse.Itinerary) != req.Duration {
		log.Printf("Day count mismatch: requested %d, generated %d", req.Duration, len(travelResponse.Itinerary))
		// 부족하면 채우기, 많으면 자르기
		travelResponse.Itinerary = adjustItineraryDays(travelResponse.Itinerary, req.Duration)
	}

	// 데이터베이스에 저장 (선택사항)
	go h.saveTravelPlan(req, travelResponse)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    travelResponse,
		"meta": fiber.Map{
			"destination": req.Destination,
			"duration":    req.Duration,
			"model":       "gemma-3-27b-it",
		},
	})
}

// GetSavedPlans 저장된 여행 계획 목록 조회
// @Summary 저장된 여행 계획 목록
// @Description 공개된 여행 계획들을 조회합니다
// @Tags travel
// @Produce json
// @Param page query int false "페이지 번호" default(1)
// @Param limit query int false "페이지당 항목 수" default(10)
// @Param destination query string false "목적지 필터"
// @Success 200 {array} models.TravelPlan "여행 계획 목록"
// @Router /api/v1/travel/plans [get]
func (h *TravelHandler) GetSavedPlans(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	destination := c.Query("destination", "")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	offset := (page - 1) * limit

	query := database.DB.Where("is_public = ?", true)

	if destination != "" {
		query = query.Where("destination ILIKE ?", "%"+destination+"%")
	}

	var plans []models.TravelPlans
	var total int64

	// 전체 개수 조회
	query.Model(&models.TravelPlans{}).Count(&total)

	// 페이징된 결과 조회
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&plans).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "여행 계획 조회 중 오류가 발생했습니다",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    plans,
		"meta": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetPlanByID 특정 여행 계획 상세 조회
// @Summary 여행 계획 상세 조회
// @Description 특정 여행 계획의 상세 정보를 조회합니다
// @Tags travel
// @Produce json
// @Param id path string true "여행 계획 ID"
// @Success 200 {object} models.TravelPlan "여행 계획 상세"
// @Failure 404 {object} map[string]interface{} "계획을 찾을 수 없음"
// @Router /api/v1/travel/plans/{id} [get]
func (h *TravelHandler) GetPlanByID(c *fiber.Ctx) error {
	planID := c.Params("id")

	var plan models.TravelPlans
	if err := database.DB.Where("id = ? AND is_public = ?", planID, true).First(&plan).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"message": "여행 계획을 찾을 수 없습니다",
		})
	}

	// 조회수 증가
	database.DB.Model(&plan).Update("view_count", plan.ViewCount+1)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    plan,
	})
}

// saveTravelPlan 여행 계획을 데이터베이스에 저장 (비동기)
func (h *TravelHandler) saveTravelPlan(req models.TravelRequest, resp models.TravelResponse) {
	planJSON, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshaling travel plan: %v", err)
		return
	}

	plan := models.TravelPlans{
		Destination: req.Destination,
		Duration:    req.Duration,
		AgeGroup:    getStringValue(req.AgeGroup),
		GroupSize:   getIntValue(req.GroupSize),
		Purpose:     getStringValue(req.Purpose),
		TravelType:  getStringValue(req.TravelType),
		PlanData:    string(planJSON),
		IsPublic:    true, // 기본적으로 공개
	}

	if err := database.DB.Create(&plan).Error; err != nil {
		log.Printf("Error saving travel plan: %v", err)
	}
}

// extractJSON 텍스트에서 JSON 부분만 추출
func extractJSON(text string) string {
	// { 로 시작하는 첫 번째 위치 찾기
	start := strings.Index(text, "{")
	if start == -1 {
		return text
	}

	// } 로 끝나는 마지막 위치 찾기
	end := strings.LastIndex(text, "}")
	if end == -1 || end <= start {
		return text
	}

	return text[start : end+1]
}

// adjustItineraryDays 일정 일수를 요청된 기간에 맞게 조정
func adjustItineraryDays(itinerary []models.DayItinerary, targetDays int) []models.DayItinerary {
	if len(itinerary) == targetDays {
		return itinerary
	}

	if len(itinerary) > targetDays {
		// 잘라내기
		return itinerary[:targetDays]
	}

	// 부족하면 마지막 날을 복사해서 채우기
	result := make([]models.DayItinerary, targetDays)
	copy(result, itinerary)

	if len(itinerary) > 0 {
		lastDay := itinerary[len(itinerary)-1]
		for i := len(itinerary); i < targetDays; i++ {
			dayItinerary := lastDay
			dayItinerary.Day = i + 1
			result[i] = dayItinerary
		}
	}

	return result
}

// Helper functions for pointer types
func getStringValue(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}

func getIntValue(ptr *int) int {
	if ptr != nil {
		return *ptr
	}
	return 0
}
