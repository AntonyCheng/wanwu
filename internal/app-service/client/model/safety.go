package model

type SensitiveWordTable struct {
	ID        int64  `gorm:"primary_key;autoIncrement"`
	CreatedAt int64  `gorm:"autoCreateTime:milli;index:idx_swt_created_at"`
	UpdatedAt int64  `gorm:"autoUpdateTime:milli"`
	UserID    string `gorm:"index:idx_swt_user_id"`
	OrgID     string `gorm:"index:idx_swt_org_id"`
	Name      string `gorm:"index:idx_swt_name"`
	Remark    string `gorm:"index:idx_swt_remark"`
	Reply     string `gorm:"index:idx_swt_reply"`
	Version   string `gorm:"index:idx_swt_version"`
}

type SensitiveWordVocabulary struct {
	ID            int64  `gorm:"primary_key;autoIncrement"`
	CreatedAt     int64  `gorm:"autoCreateTime:milli;index:idx_swv_created_at"`
	UpdatedAt     int64  `gorm:"autoUpdateTime:milli"`
	UserID        string `gorm:"index:idx_swv_user_id"`
	OrgID         string `gorm:"index:idx_swv_org_id"`
	TableID       string `gorm:"index:idx_swv_table_id"`
	SensitiveType string `gorm:"index:idx_swv_sensitive_type"`
	Content       string `gorm:"index:idx_swv_content"`
}
