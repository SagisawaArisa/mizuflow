package model

type SDKClient struct {
	ID     uint64 `gorm:"primaryKey"`
	AppID  string `gorm:"size:64;not null"`
	APIKey string `gorm:"size:64;not null"`
	Env    string `gorm:"size:32;default:dev"`
	Status int    `gorm:"default:1"`
}
