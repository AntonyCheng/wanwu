package model

type ApiKey struct {
	ID        uint32 `gorm:"primary_key"`
	CreatedAt int64  `gorm:"autoCreateTime:milli;index:idx_api_key_created_at"`
	UpdatedAt int64  `gorm:"autoUpdateTime:milli"`
	// 组织ID
	OrgID string `gorm:"index:idx_api_key_org_id"`
	// 用户ID
	UserID string `gorm:"index:idx_api_key_user_id"`
	// 应用ID
	AppID string `gorm:"index:idx_api_key_app_id"`
	// 应用类型
	AppType string `gorm:"index:idx_api_key_app_type"`
	// Api Key
	ApiKey string `gorm:"index:idx_api_key_api_key"`
}
