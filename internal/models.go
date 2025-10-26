package internal

import (
	"time"
)

type URL struct {
	ID        int64  `gorm:"primaryKey;type:bigint"`
	ShortCode string `gorm:"type:varchar(12);uniqueIndex;not null"`
	LongURL   string `gorm:"type:text;index;not null"`
	CreatedAt time.Time
}
