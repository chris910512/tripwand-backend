package places

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

type PlacesRequest struct {
	Input    string `json:"input"`
	Language string `json:"language"`
	Types    string `json:"types"`
}

type PlacesResponse struct {
	Predictions []Prediction `json:"predictions"`
	Status      string       `json:"status"`
}

type Prediction struct {
	PlaceID               string                `json:"place_id"`
	Description           string                `json:"description"`
	StructuredFormatting  StructuredFormatting  `json:"structured_formatting"`
}

type StructuredFormatting struct {
	MainText      string `json:"main_text"`
	SecondaryText string `json:"secondary_text"`
}

// New Places API response structures
type NewPlacesResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

type Suggestion struct {
	PlacePrediction PlacePrediction `json:"placePrediction"`
}

type PlacePrediction struct {
	PlaceID string `json:"placeId"`
	Text    Text   `json:"text"`
}

type Text struct {
	Text string `json:"text"`
}

func NewClient() *Client {
	return &Client{
		apiKey: os.Getenv("GOOGLE_PLACES_API_KEY"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: "https://maps.googleapis.com/maps/api/place",
	}
}

func (c *Client) SearchPlaces(req PlacesRequest) (*PlacesResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_PLACES_API_KEY environment variable is not set")
	}

	// Use the new Places API (New) endpoint
	requestURL := "https://places.googleapis.com/v1/places:autocomplete"

	// Build request body for the new API
	requestBody := map[string]interface{}{
		"input":        req.Input,
		"languageCode": req.Language,
	}

	// Add location bias for cities (new API uses different format)
	if req.Types == "cities" {
		requestBody["includedPrimaryTypes"] = []string{"locality", "administrative_area_level_1"}
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request with proper headers for new API
	httpReq, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Goog-Api-Key", c.apiKey)
	httpReq.Header.Set("X-Goog-FieldMask", "suggestions.placePrediction.placeId,suggestions.placePrediction.text")

	// Make HTTP request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to Google Places API: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google Places API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse new API response format
	var newAPIResponse NewPlacesResponse
	if err := json.Unmarshal(body, &newAPIResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}

	// Convert new API response to legacy format for compatibility
	placesResponse := &PlacesResponse{
		Status:      "OK",
		Predictions: make([]Prediction, 0),
	}

	for _, suggestion := range newAPIResponse.Suggestions {
		if suggestion.PlacePrediction.PlaceID != "" {
			prediction := Prediction{
				PlaceID:     suggestion.PlacePrediction.PlaceID,
				Description: suggestion.PlacePrediction.Text.Text,
				StructuredFormatting: StructuredFormatting{
					MainText:      suggestion.PlacePrediction.Text.Text,
					SecondaryText: "",
				},
			}
			placesResponse.Predictions = append(placesResponse.Predictions, prediction)
		}
	}

	return placesResponse, nil
}