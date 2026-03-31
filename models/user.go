package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	TelegramID int64 `gorm:"uniqueIndex"`
	FirstName  string
	LastName   string
	Phone      string
}

// Suhbat sessiyasi (har bir yangi suhbat)
type ChatSession struct {
	gorm.Model
	TelegramID int64  `gorm:"index"`
	SessionID  string `gorm:"uniqueIndex"`
	Title      string // Suhbat sarlavhasi (birinchi xabardan olinadi)
}

// Har bir xabar
type ChatMessage struct {
	gorm.Model
	SessionID string `gorm:"index"`
	Role      string // "user" yoki "assistant"
	Content   string `gorm:"type:text"`
}
