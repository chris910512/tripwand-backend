package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// TravelRequest 프론트엔드로부터 받는 여행 요청
type TravelRequest struct {
	Destination string  `json:"destination" validate:"required" example:"부산"`
	Duration    int     `json:"duration" validate:"required,min=1,max=30" example:"3"`
	AgeGroup    *string `json:"age_group,omitempty" example:"20대"`
	GroupSize   *int    `json:"group_size,omitempty" validate:"omitempty,min=1,max=50" example:"2"`
	Purpose     *string `json:"purpose,omitempty" example:"힐링과 휴식"`
	TravelType  *string `json:"travel_type,omitempty" example:"여유로운 여행"`
}

// ActivityPeriod 하루 중 시간대별 활동
type ActivityPeriod struct {
	Summary string `json:"summary" example:"부산 해운대 해변 산책"`
	Detail  string `json:"detail" example:"새벽 일출을 보며 해변을 걷고, 근처 카페에서 아침 식사를 즐깁니다."`
}

// DayItinerary 하루 일정
type DayItinerary struct {
	Day       int            `json:"day" example:"1"`
	Morning   ActivityPeriod `json:"morning"`
	Afternoon ActivityPeriod `json:"afternoon"`
	Evening   ActivityPeriod `json:"evening"`
	Night     ActivityPeriod `json:"night"`
}

// TravelResponse 프론트엔드로 반환하는 여행 일정 응답
type TravelResponse struct {
	Itinerary     []DayItinerary `json:"itinerary"`
	EstimatedCost int            `json:"estimated_cost" example:"500000"`
	Cautions      []string       `json:"cautions" example:"날씨 확인 필수,예약 미리 하기"`
}

// TravelPlans 데이터베이스에 저장할 여행 계획 (선택사항)
type TravelPlans struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      *uint          `gorm:"index" json:"user_id"` // nullable - 비회원도 사용 가능
	Destination string         `gorm:"size:255;not null" json:"destination"`
	Duration    int            `gorm:"not null" json:"duration"`
	AgeGroup    string         `gorm:"size:50" json:"age_group"`
	GroupSize   int            `json:"group_size"`
	Purpose     string         `gorm:"size:100" json:"purpose"`
	TravelType  string         `gorm:"size:100" json:"travel_type"`
	PlanData    string         `gorm:"type:text" json:"plan_data"` // JSON 형태로 저장된 여행 계획
	IsPublic    bool           `gorm:"default:false" json:"is_public"`
	ViewCount   int            `gorm:"default:0" json:"view_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (TravelPlans) TableName() string {
	return "travel_plans"
}

// GemmaPromptData Gemma에 전달할 프롬프트 데이터
type GemmaPromptData struct {
	Destination string
	Duration    int
	AgeGroup    string
	GroupSize   string
	Purpose     string
	TravelType  string
}

// ToGemmaPrompt TravelRequest를 Gemma 프롬프트로 변환
func (tr *TravelRequest) ToGemmaPrompt() string {
	data := GemmaPromptData{
		Destination: tr.Destination,
		Duration:    tr.Duration,
		AgeGroup:    getStringValue(tr.AgeGroup, "연령대 미지정"),
		GroupSize:   getGroupSizeText(tr.GroupSize),
		Purpose:     getStringValue(tr.Purpose, "일반적인 관광"),
		TravelType:  getStringValue(tr.TravelType, "균형잡힌 여행"),
	}

	prompt := fmt.Sprintf(`%s을(를) %d일 동안 여행할 예정입니다. 
나이대는 %s이고, %s이서 여행합니다. 
여행 목적은 "%s"이며, 여행 스타일은 "%s"입니다.

다음 JSON 형식으로 정확히 답변해주세요. 다른 설명이나 부가 텍스트 없이 오직 JSON만 반환하세요:

{
  "itinerary": [
    {
      "day": 1,
      "morning": {
        "summary": "아침 활동 요약",
        "detail": "아침 활동 상세 설명"
      },
      "afternoon": {
        "summary": "오후 활동 요약", 
        "detail": "오후 활동 상세 설명"
      },
      "evening": {
        "summary": "저녁 활동 요약",
        "detail": "저녁 활동 상세 설명"  
      },
      "night": {
        "summary": "밤 활동 요약",
        "detail": "밤 활동 상세 설명"
      }
    }
  ],
  "estimated_cost": 예상비용(숫자만),
  "cautions": ["주의사항1", "주의사항2"]
}

각 일차별로 현실적이고 구체적인 일정을 만들어주세요. 예상 비용은 1인 기준 한국 원화로 계산해주세요.`,
		data.Destination, data.Duration, data.AgeGroup, data.GroupSize, data.Purpose, data.TravelType)

	return prompt
}

// 헬퍼 함수들
func getStringValue(ptr *string, defaultValue string) string {
	if ptr != nil && *ptr != "" {
		return *ptr
	}
	return defaultValue
}

func getGroupSizeText(groupSize *int) string {
	if groupSize == nil {
		return "1명"
	}

	if *groupSize == 1 {
		return "혼자"
	}
	return fmt.Sprintf("%d명", *groupSize)
}

// BeforeCreate is not needed for uint primary key as GORM handles auto-increment
// func (tp *TravelPlans) BeforeCreate(tx *gorm.DB) error {
// 	// GORM will auto-generate uint ID
// 	return nil
// }
