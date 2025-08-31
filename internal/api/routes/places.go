package routes

import (
	"github.com/gofiber/fiber/v2"
	"tripwand-backend/internal/api/handlers"
)

func SetupPlacesRoutes(api fiber.Router) {
	placesHandler := handlers.NewPlacesHandler()

	// Places 관련 라우트
	placesGroup := api.Group("/places")
	
	// POST /api/v1/places/search - Google Places Autocomplete 검색
	placesGroup.Post("/search", placesHandler.SearchPlaces)
}