package orm

import (
	"context"
	"encoding/json"
	"errors"
	knowledgebase_keywords_service "github.com/UnicomAI/wanwu/api/proto/knowledgebase-keywords-service"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/client/model"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/client/orm/sqlopt"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/pkg/db"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/pkg/util"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/service"
	"github.com/UnicomAI/wanwu/pkg/log"
	"gorm.io/gorm"
)

const (
	KeywordsAdd    = "add"    // 添加关键词
	KeywordsDelete = "delete" // 删除关键词
	KeywordsUpdate = "update" // 更新关键词
)

// GetKeywordsList 根据 userId 和 orgId 查询关键词列表
func GetKeywordsList(ctx context.Context, req *knowledgebase_keywords_service.GetKnowledgeKeywordsListReq) ([]*model.KnowledgeKeywords, int64, error) {
	var keywordsList []*model.KnowledgeKeywords
	tx := sqlopt.SQLOptions(sqlopt.WithPermit(req.Identity.OrgId, req.Identity.UserId), sqlopt.WithNameOrAliasLike(req.Name)).
		Apply(db.GetHandle(ctx), &model.KnowledgeKeywords{})
	var total int64
	err := tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	limit := req.PageSize
	offset := req.PageSize * (req.PageNum - 1)
	err = tx.Order("updated_at desc").Limit(int(limit)).Offset(int(offset)).Find(&keywordsList).Error
	if err != nil {
		return nil, 0, err
	}
	return keywordsList, total, nil
}

// GetKeywordsById 根据关键词id查询关键词
func GetKeywordsById(ctx context.Context, id uint32) (*model.KnowledgeKeywords, error) {
	var keywords model.KnowledgeKeywords
	err := sqlopt.SQLOptions(sqlopt.WithID(id)).
		Apply(db.GetHandle(ctx), &model.KnowledgeKeywords{}).First(&keywords).Error
	if err != nil {
		return nil, err
	}
	return &keywords, nil
}

// CheckRepeatedKeywords 查询用户是否存在同名关键词设置
func CheckRepeatedKeywords(ctx context.Context, req *knowledgebase_keywords_service.CreateKnowledgeKeywordsReq) error {
	var keywordsList []*model.KnowledgeKeywords
	// 查找同名关键词列表
	err := sqlopt.SQLOptions(sqlopt.WithPermit(req.Identity.OrgId, req.Identity.UserId), sqlopt.WithName(req.Name)).
		Apply(db.GetHandle(ctx), &model.KnowledgeKeywords{}).Find(&keywordsList).Error

	if err != nil {
		return err
	}
	// 已有关键词，检查同名知识库
	if len(keywordsList) != 0 {
		for _, kw := range keywordsList {
			dbKnowledgeIds, err := jsonToList(kw.KnowledgeBaseIds)
			if err != nil {
				return err
			}
			if util.HasIntersection(dbKnowledgeIds, req.KnowledgeBaseIds) {
				log.Errorf("有同名关键词")
				return gorm.ErrDuplicatedKey
			}
		}
	}
	return nil
}

// CreateKeywords 创建关键词
func CreateKeywords(ctx context.Context, keywords *model.KnowledgeKeywords) error {
	return db.GetHandle(ctx).Transaction(func(tx *gorm.DB) error {
		// 创建关键词
		err := db.GetHandle(ctx).Create(keywords).Error
		if err != nil {
			return err
		}
		keywordsParams, err := buildOperateKeywordsParams(keywords, KeywordsAdd)
		if err != nil {
			return err
		}
		// 同步RAG
		return service.RagOperateKeywords(ctx, keywordsParams)
	})
}

// DeleteKeywords 删除知识库关键词
func DeleteKeywords(ctx context.Context, id uint32) error {
	// 获取关键词
	keywords, err := GetKeywordsById(ctx, id)
	if err != nil {
		return err
	}
	return db.GetHandle(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除关键词
		err = tx.Unscoped().Model(&model.KnowledgeKeywords{}).Where("id = ?", id).Delete(&model.KnowledgeKeywords{}).Error
		if err != nil {
			log.Errorf("DeleteKnowledgeKeywords err: %v", err)
			return err
		}
		// 同步RAG
		keywordsParams, err := buildOperateKeywordsParams(keywords, KeywordsDelete)
		if err != nil {
			return err
		}
		return service.RagOperateKeywords(ctx, keywordsParams)
	})
}

// UpdateKeywords 更新知识库关键词
func UpdateKeywords(ctx context.Context, keywords *model.KnowledgeKeywords) error {
	return db.GetHandle(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除关键词
		err := db.GetHandle(ctx).Model(&model.KnowledgeKeywords{}).Where("id = ?", keywords.Id).Updates(keywords).Debug().Error
		if err != nil {
			log.Errorf("UpdateKeywords err: %v", err)
			return err
		}
		// 同步RAG
		keywordsParams, err := buildOperateKeywordsParams(keywords, KeywordsUpdate)
		if err != nil {
			return err
		}
		return service.RagOperateKeywords(ctx, keywordsParams)
	})
}

func buildOperateKeywordsParams(keywords *model.KnowledgeKeywords, action string) (*service.RagOperateKeywordsParams, error) {
	// 反序列化id列表
	knowledgeIds, err := jsonToList(keywords.KnowledgeBaseIds)
	if err != nil {
		return nil, err
	}
	if len(knowledgeIds) == 0 {
		return nil, errors.New("knowledgeIds length can not be zero")
	}
	return &service.RagOperateKeywordsParams{
		Id:               keywords.Id,
		UserId:           keywords.UserId,
		Action:           action,
		Name:             keywords.Name,
		Alias:            []string{keywords.Alias},
		KnowledgeBaseIds: knowledgeIds,
	}, nil
}

func jsonToList(knowledgeBaseIdStr string) ([]string, error) {
	var knowledgeIds []string
	err := json.Unmarshal([]byte(knowledgeBaseIdStr), &knowledgeIds)
	if err != nil {
		log.Errorf("反序列化错误: %v, 原始字符串: %s", err, knowledgeBaseIdStr)
		return nil, err
	}
	return knowledgeIds, nil
}
