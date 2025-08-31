package handlers

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"tripwand-backend/internal/places"
)

type PlacesHandler struct {
	placesClient *places.Client
}

func NewPlacesHandler() *PlacesHandler {
	return &PlacesHandler{
		placesClient: places.NewClient(),
	}
}

type PlacesSearchRequest struct {
	Input    string `json:"input"`
	Language string `json:"language"`
	Types    string `json:"types"`
}

type PlacesSearchResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
	Error   string      `json:"error,omitempty"`
}

func (h *PlacesHandler) SearchPlaces(c *fiber.Ctx) error {
	log.Println("🔍 Places Search API called")
	
	// 핸들러와 클라이언트 nil 체크
	if h == nil {
		log.Println("❌ PlacesHandler is nil")
		return c.Status(500).JSON(PlacesSearchResponse{
			Success: false,
			Data:    nil,
			Message: "Internal server error: handler is nil",
			Error:   "HANDLER_NIL",
		})
	}
	
	if h.placesClient == nil {
		log.Println("❌ Places client is nil")
		return c.Status(500).JSON(PlacesSearchResponse{
			Success: false,
			Data:    nil,
			Message: "Internal server error: Places client not initialized",
			Error:   "CLIENT_NIL",
		})
	}

	var req PlacesSearchRequest

	if err := c.BodyParser(&req); err != nil {
		log.Printf("❌ Body parsing error: %v", err)
		return c.Status(400).JSON(PlacesSearchResponse{
			Success: false,
			Data:    nil,
			Message: "Invalid request body",
			Error:   "INVALID_JSON",
		})
	}
	
	log.Printf("📥 Request: input=%s, language=%s, types=%s", req.Input, req.Language, req.Types)

	// 입력 검증
	if len(strings.TrimSpace(req.Input)) < 2 {
		return c.Status(400).JSON(PlacesSearchResponse{
			Success: false,
			Data:    nil,
			Message: "Input must be at least 2 characters long",
			Error:   "INVALID_INPUT",
		})
	}

	if req.Language == "" {
		req.Language = "ko" // 기본값
	}

	if req.Types == "" {
		req.Types = "cities" // 기본값
	}

	// Places API 호출
	placesReq := places.PlacesRequest{
		Input:    strings.TrimSpace(req.Input),
		Language: req.Language,
		Types:    req.Types,
	}

	log.Printf("📤 Calling Places API with: %+v", placesReq)
	result, err := h.placesClient.SearchPlaces(placesReq)
	if err != nil {
		log.Printf("❌ Places API error: %v", err)
		// API 키 오류 처리
		if strings.Contains(err.Error(), "GOOGLE_PLACES_API_KEY") {
			return c.Status(500).JSON(PlacesSearchResponse{
				Success: false,
				Data:    nil,
				Message: "Google Places API key is not configured",
				Error:   "API_KEY_ERROR",
			})
		}

		// Google Places API 오류 처리 (result가 nil일 수 있으므로 안전하게 처리)
		errorData := fiber.Map{
			"error_message": err.Error(),
		}
		if result != nil {
			errorData["status"] = result.Status
		}

		return c.Status(500).JSON(PlacesSearchResponse{
			Success: false,
			Data:    errorData,
			Message: "Google Places API returned an error",
			Error:   "PLACES_API_ERROR",
		})
	}

	// 성공 응답
	return c.JSON(PlacesSearchResponse{
		Success: true,
		Data:    result,
		Message: "Places search completed successfully",
	})
}