package model

import "time"

// Reminder represents a saved reminder for a WhatsApp user.
type Reminder struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"index;not null"`
	Content   string    `gorm:"type:text;not null"`
	Priority  int       `gorm:"not null"`
	Summary   string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
