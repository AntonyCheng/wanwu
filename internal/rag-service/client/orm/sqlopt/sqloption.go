package sqlopt

import (
	"gorm.io/gorm"
)

type sqlOptions []SQLOption

func SQLOptions(opts ...SQLOption) sqlOptions {
	return opts
}

func (s sqlOptions) Apply(db *gorm.DB) *gorm.DB {
	for _, opt := range s {
		db = opt.Apply(db)
	}
	return db
}

type SQLOption interface {
	Apply(db *gorm.DB) *gorm.DB
}

type funcSQLOption func(db *gorm.DB) *gorm.DB

func (f funcSQLOption) Apply(db *gorm.DB) *gorm.DB {
	return f(db)
}

func WithOrgID(orgID string) SQLOption {
	return funcSQLOption(func(db *gorm.DB) *gorm.DB {
		if orgID != "" {
			return db.Where("org_id = ?", orgID)
		}
		return db
	})
}

func WithUserID(userID string) SQLOption {
	return funcSQLOption(func(db *gorm.DB) *gorm.DB {
		if userID != "" {
			return db.Where("user_id = ?", userID)
		}
		return db
	})
}

func WithRagID(ragID string) SQLOption {
	return funcSQLOption(func(db *gorm.DB) *gorm.DB {
		if ragID != "" {
			return db.Where("rag_id = ?", ragID)
		}
		return db
	})
}

func LikeBriefName(briefName string) SQLOption {
	return funcSQLOption(func(db *gorm.DB) *gorm.DB {
		if briefName != "" {
			return db.Where("brief_name LIKE ?", "%"+briefName+"%")
		}
		return db
	})
}

func InRagIds(ragIds []string) SQLOption {
	return funcSQLOption(func(db *gorm.DB) *gorm.DB {
		if len(ragIds) > 0 {
			return db.Where("rag_id IN ?", ragIds)
		}
		return db
	})
}
