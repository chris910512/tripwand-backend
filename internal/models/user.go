package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// User 모델 - 사용자 정보
type User struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	Email           string         `gorm:"uniqueIndex;not null" json:"email"`
	Nickname        string         `gorm:"size:100" json:"nickname"`
	ProfileImageURL string         `gorm:"size:500" json:"profile_image_url"`
	IsActive        bool           `gorm:"default:true" json:"is_active"`
	LastLoginAt     *time.Time     `json:"last_login_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	OAuthAccounts []OAuthAccount `gorm:"foreignKey:UserID" json:"oauth_accounts,omitempty"`
	TravelPlans   []TravelPlans  `gorm:"foreignKey:UserID" json:"travels,omitempty"`
}

// OAuthAccount 모델 - OAuth 제공자별 계정 정보
type OAuthAccount struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"not null;index" json:"user_id"`
	Provider     string     `gorm:"size:50;not null;index:idx_provider_provider_id,unique" json:"provider"` // google, apple, naver, kakao
	ProviderID   string     `gorm:"size:255;not null;index:idx_provider_provider_id,unique" json:"provider_id"`
	AccessToken  string     `gorm:"size:500" json:"-"` // 보안상 JSON 응답에서 제외
	RefreshToken string     `gorm:"size:500" json:"-"` // 보안상 JSON 응답에서 제외
	TokenExpiry  *time.Time `json:"token_expiry"`
	Email        string     `gorm:"size:255" json:"email"`
	Name         string     `gorm:"size:255" json:"name"`
	Picture      string     `gorm:"size:500" json:"picture"`
	Locale       string     `gorm:"size:10" json:"locale"`
	RawData      string     `gorm:"type:text" json:"-"` // OAuth 제공자로부터 받은 원본 데이터 (JSON)
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// UserSession 모델 - 사용자 세션 관리
type UserSession struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"not null;index" json:"user_id"`
	AccessToken  string    `gorm:"size:500;uniqueIndex" json:"access_token"`
	RefreshToken string    `gorm:"size:500" json:"refresh_token"`
	DeviceInfo   string    `gorm:"size:255" json:"device_info"`
	IPAddress    string    `gorm:"size:45" json:"ip_address"`
	UserAgent    string    `gorm:"size:500" json:"user_agent"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName 메서드들 - 테이블 이름 지정
func (User) TableName() string {
	return "users"
}

func (OAuthAccount) TableName() string {
	return "oauth_accounts"
}

func (UserSession) TableName() string {
	return "user_sessions"
}

// BeforeCreate hooks
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Nickname == "" && u.Email != "" {
		// 이메일에서 @ 앞부분을 기본 닉네임으로 사용
		if idx := strings.Index(u.Email, "@"); idx > 0 {
			u.Nickname = u.Email[:idx]
		}
	}
	return nil
}

// User 관련 메서드
func (u *User) UpdateLastLogin(db *gorm.DB) error {
	now := time.Now()
	return db.Model(u).Update("last_login_at", now).Error
}

// FindOrCreateUserByOAuth - OAuth 정보로 사용자 찾기 또는 생성
func FindOrCreateUserByOAuth(db *gorm.DB, provider, providerID, email, name, picture string) (*User, error) {
	var oauthAccount OAuthAccount

	// 기존 OAuth 계정 확인
	err := db.Where("provider = ? AND provider_id = ?", provider, providerID).First(&oauthAccount).Error

	if err == nil {
		// 기존 사용자 반환
		var user User
		if err := db.First(&user, oauthAccount.UserID).Error; err != nil {
			return nil, err
		}
		return &user, nil
	}

	// 새 사용자 생성
	if err == gorm.ErrRecordNotFound {
		// 이메일로 기존 사용자 확인
		var user User
		err = db.Where("email = ?", email).First(&user).Error

		if err == gorm.ErrRecordNotFound {
			// 완전히 새로운 사용자 생성
			user = User{
				Email:           email,
				Nickname:        name,
				ProfileImageURL: picture,
			}
			if err := db.Create(&user).Error; err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}

		// OAuth 계정 연결
		oauthAccount = OAuthAccount{
			UserID:     user.ID,
			Provider:   provider,
			ProviderID: providerID,
			Email:      email,
			Name:       name,
			Picture:    picture,
		}
		if err := db.Create(&oauthAccount).Error; err != nil {
			return nil, err
		}

		return &user, nil
	}

	return nil, err
}
