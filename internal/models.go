package internal

import (
	"time"
)

type URL struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	ShortCode string `gorm:"type:varchar(12);uniqueIndex;not null"`
	LongURL   string `gorm:"type:text;index;not null"`
	CreatedAt time.Time
}
