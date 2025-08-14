package orm

import (
	"context"

	errs "github.com/UnicomAI/wanwu/api/proto/err-code"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/client/model"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/client/orm/sqlopt"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/pkg/db"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/pkg/util"
	"github.com/UnicomAI/wanwu/pkg/log"
)

// SelectKnowledgeSplitterList 查询知识库分隔符列表
func SelectKnowledgeSplitterList(ctx context.Context, userId, orgId, name string) ([]*model.KnowledgeSplitter, error) {
	var knowledgeSplitterList []*model.KnowledgeSplitter
	err := sqlopt.SQLOptions(sqlopt.WithPermit(orgId, userId), sqlopt.WithName(name)).
		Apply(db.GetHandle(ctx), &model.KnowledgeSplitter{}).
		Order("create_at desc").
		Find(&knowledgeSplitterList).
		Error
	if err != nil {
		return nil, err
	}
	return knowledgeSplitterList, nil
}

// SelectKnowledgeSplitterDetail 查询知识库分隔符详情
func SelectKnowledgeSplitterDetail(ctx context.Context, userId, orgId, splitterId string) (*model.KnowledgeSplitter, error) {
	var knowledgeSplitter model.KnowledgeSplitter
	err := sqlopt.SQLOptions(sqlopt.WithPermit(orgId, userId), sqlopt.WithSplitterID(splitterId)).
		Apply(db.GetHandle(ctx), &model.KnowledgeSplitter{}).
		First(&knowledgeSplitter).Error
	if err != nil {
		return nil, err
	}
	return &knowledgeSplitter, nil
}

func CheckSameKnowledgeSplitterName(ctx context.Context, userId, orgId, name string) error {
	var count int64
	err := sqlopt.SQLOptions(sqlopt.WithPermit(orgId, userId), sqlopt.WithName(name)).
		Apply(db.GetHandle(ctx), &model.KnowledgeSplitter{}).
		Count(&count).Error
	if err != nil {
		log.Errorf("KnowledgeSplitterNameExist userId %s name %s err: %v", userId, name, err)
		return util.ErrCode(errs.Code_KnowledgeSplitterDuplicateName)
	}
	if count > 0 {
		return util.ErrCode(errs.Code_KnowledgeSplitterDuplicateName)
	}
	return nil
}

// CreateKnowledgeSplitter 创建知识库标签
func CreateKnowledgeSplitter(ctx context.Context, knowledgeSplitter *model.KnowledgeSplitter) error {
	return db.GetHandle(ctx).Create(knowledgeSplitter).Error
}

// UpdateKnowledgeSplitter 更新知识库标签
func UpdateKnowledgeSplitter(ctx context.Context, name, value string, id uint32) error {
	var updateParams = map[string]interface{}{
		"name":  name,
		"value": value,
	}
	return db.GetHandle(ctx).Model(&model.KnowledgeSplitter{}).Where("id = ?", id).Updates(updateParams).Error
}

// DeleteKnowledgeSplitter 删除知识库标签
func DeleteKnowledgeSplitter(ctx context.Context, id uint32) error {
	err := db.GetHandle(ctx).Unscoped().Model(&model.KnowledgeSplitter{}).Where("id = ?", id).Delete(&model.KnowledgeSplitter{}).Error
	if err != nil {
		log.Errorf("DeleteKnowledgeSplitter err: %v", err)
		return err
	}
	return nil
}
