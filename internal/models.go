package internal

import (
	"time"
)

type URL struct {
	ID        int64  `gorm:"primaryKey;type:bigint"`
	ShortCode string `gorm:"type:varchar(12);uniqueIndex;not null"`
	LongURL   string `gorm:"type:text;index;not null"`
	CreatedAt time.Time
	Analytics URLAnalytics `gorm:"foreignKey:ShortCode;references:ShortCode;constraint:OnDelete:CASCADE"`
}

type URLAnalytics struct {
	ShortCode  string `gorm:"primaryKey;type:varchar(12)"`
	ClickCount int64  `gorm:"default:0;not null"`
}
