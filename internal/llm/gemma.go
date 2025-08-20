// internal/llm/gemma.go
package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GemmaClient Google AI Studio Gemma 클라이언트
type GemmaClient struct {
	client *genai.Client
	model  *genai.GenerativeModel
	ctx    context.Context
}

// ChatMessage 채팅 메시지 구조체
type ChatMessage struct {
	Role    string `json:"role"` // "user" 또는 "model"
	Content string `json:"content"`
}

// ChatRequest 채팅 요청 구조체
type ChatRequest struct {
	Message     string        `json:"message"`
	History     []ChatMessage `json:"history,omitempty"`
	Temperature float32       `json:"temperature,omitempty"`
	MaxTokens   int32         `json:"max_tokens,omitempty"`
}

// ChatResponse 채팅 응답 구조체
type ChatResponse struct {
	Response string `json:"response"`
	Model    string `json:"model"`
}

// GenerateRequest 텍스트 생성 요청 구조체
type GenerateRequest struct {
	Prompt      string  `json:"prompt"`
	Temperature float32 `json:"temperature,omitempty"`
	MaxTokens   int32   `json:"max_tokens,omitempty"`
}

// GenerateResponse 텍스트 생성 응답 구조체
type GenerateResponse struct {
	GeneratedText string `json:"generated_text"`
	Model         string `json:"model"`
	Prompt        string `json:"prompt"`
}

// NewGemmaClient Gemma 클라이언트 생성
func NewGemmaClient() (*GemmaClient, error) {
	ctx := context.Background()

	apiKey := os.Getenv("GOOGLE_AI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_AI_API_KEY environment variable is required")
	}

	// Google AI 클라이언트 초기화
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	// Gemma 3 27B 모델 선택 (최신 및 가장 성능이 좋은 모델)
	model := client.GenerativeModel("gemma-3-27b-it")

	// 기본 설정
	model.SetTemperature(0.7)
	model.SetMaxOutputTokens(1000)
	model.SetTopP(0.9)
	model.SetTopK(40)

	// 안전 설정 (한국어 콘텐츠 지원)
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockMediumAndAbove,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockMediumAndAbove,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockMediumAndAbove,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockMediumAndAbove,
		},
	}

	return &GemmaClient{
		client: client,
		model:  model,
		ctx:    ctx,
	}, nil
}

// Chat 대화형 채팅 (히스토리 포함)
func (g *GemmaClient) Chat(req ChatRequest) (*ChatResponse, error) {
	// 채팅 세션 시작
	session := g.model.StartChat()

	// 히스토리가 있으면 추가
	if len(req.History) > 0 {
		for _, msg := range req.History {
			var role string
			if msg.Role == "user" {
				role = "user"
			} else {
				role = "model"
			}

			session.History = append(session.History, &genai.Content{
				Role: role,
				Parts: []genai.Part{
					genai.Text(msg.Content),
				},
			})
		}
	}

	// 설정 적용
	if req.Temperature > 0 {
		g.model.SetTemperature(req.Temperature)
	}
	if req.MaxTokens > 0 {
		g.model.SetMaxOutputTokens(req.MaxTokens)
	}

	// 메시지 전송
	resp, err := session.SendMessage(g.ctx, genai.Text(req.Message))
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// 응답 파싱
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response generated")
	}

	responseText := ""
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			responseText += string(text)
		}
	}

	return &ChatResponse{
		Response: responseText,
		Model:    "gemma-3-27b-it",
	}, nil
}

// Generate 단순 텍스트 생성
func (g *GemmaClient) Generate(req GenerateRequest) (*GenerateResponse, error) {
	// 임시 모델 복사본 생성 (설정 변경을 위해)
	tempModel := g.client.GenerativeModel("gemma-3-27b-it")

	// 설정 적용
	if req.Temperature > 0 {
		tempModel.SetTemperature(req.Temperature)
	} else {
		tempModel.SetTemperature(0.7)
	}

	if req.MaxTokens > 0 {
		tempModel.SetMaxOutputTokens(req.MaxTokens)
	} else {
		tempModel.SetMaxOutputTokens(1000)
	}

	// 텍스트 생성
	resp, err := tempModel.GenerateContent(g.ctx, genai.Text(req.Prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	// 응답 파싱
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content generated")
	}

	generatedText := ""
	for _, part := range resp.Candidates[0].Content.Parts {
		if text, ok := part.(genai.Text); ok {
			generatedText += string(text)
		}
	}

	return &GenerateResponse{
		GeneratedText: generatedText,
		Model:         "gemma-3-27b-it",
		Prompt:        req.Prompt,
	}, nil
}

// Close 클라이언트 종료
func (g *GemmaClient) Close() error {
	return g.client.Close()
}

// GetAvailableModels 사용 가능한 모델 목록 조회
func (g *GemmaClient) GetAvailableModels() ([]string, error) {
	// Gemma 3 모델 목록 (2025년 현재)
	models := []string{
		"gemma-3-1b-it",  // 1B 파라미터 (경량)
		"gemma-3-4b-it",  // 4B 파라미터 (중간)
		"gemma-3-12b-it", // 12B 파라미터 (고성능)
		"gemma-3-27b-it", // 27B 파라미터 (최고성능)
	}

	return models, nil
}

// SwitchModel 모델 변경
func (g *GemmaClient) SwitchModel(modelName string) error {
	// 모델 유효성 검증
	availableModels, _ := g.GetAvailableModels()
	isValid := false
	for _, model := range availableModels {
		if model == modelName {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid model name: %s", modelName)
	}

	// 새 모델로 변경
	g.model = g.client.GenerativeModel(modelName)

	// 기본 설정 재적용
	g.model.SetTemperature(0.7)
	g.model.SetMaxOutputTokens(1000)
	g.model.SetTopP(0.9)
	g.model.SetTopK(40)

	return nil
}
