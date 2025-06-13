package import_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/UnicomAI/wanwu/internal/knowledge-service/client/model"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/client/orm"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/pkg/db"
	"github.com/UnicomAI/wanwu/pkg/log"
	"github.com/UnicomAI/wanwu/pkg/util"
	"gorm.io/gorm"
)

var docImportServiceMap = make(map[int]DocImportService)

func AddDocImportService(service DocImportService) {
	docImportServiceMap[service.ImportType()] = service
}

// DoDocImport 执行文件导入
func DoDocImport(ctx context.Context, task *model.KnowledgeImportTask) (resultList []*model.DocInfo, err error) {
	defer util.PrintPanicStackWithCall(func(panicOccur bool, err2 error) {
		if panicOccur {
			log.Errorf("do doc import task panic: %v", err2)
			err = fmt.Errorf("文件导入异常")
		}
		var status = model.KnowledgeImportFinish
		var errMsg string
		if err != nil {
			status = model.KnowledgeImportError
			errMsg = err.Error()
		}
		//更新状态和数量
		err = db.GetHandle(ctx).Transaction(func(tx *gorm.DB) error {
			err = orm.UpdateKnowledgeImportTaskStatus(ctx, tx, task.Id, status, errMsg)
			if err != nil {
				return err
			}
			if len(resultList) > 0 {
				return orm.UpdateKnowledgeFileInfo(tx, task.KnowledgeId, resultList)
			}
			return nil
		})
	})

	//1.获取服务service
	docImportService, ok := docImportServiceMap[task.ImportType]
	if !ok {
		log.Errorf("DoDocAnalyze not found import type %d", task.ImportType)
		//没找到处理器不算处理错误
		return nil, errors.New("DoDocAnalyze not found import type")
	}
	var importDocInfo = model.DocImportInfo{}
	err = json.Unmarshal([]byte(task.DocInfo), &importDocInfo)
	if err != nil {
		log.Errorf("Unmarshal fail %v", err)
		return nil, err
	}
	//2.更新导入任务状态
	err = orm.UpdateKnowledgeImportTaskStatus(ctx, nil, task.Id, model.KnowledgeImportAnalyze, "")
	if err != nil {
		log.Errorf("Update fail %v", err)
		return nil, err
	}
	docList, err := docImportService.AnalyzeDoc(ctx, task, &importDocInfo)
	if err != nil {
		log.Errorf("Analyze fail %v", err)
		return nil, err
	}
	if len(docList) == 0 {
		return make([]*model.DocInfo, 0), nil
	}
	//3.执行文件校验
	docCheckList, err := docImportService.CheckDoc(ctx, task, docList)
	if err != nil {
		log.Errorf("CheckDoc fail %v", err)
		return nil, err
	}
	if len(docCheckList) == 0 {
		return make([]*model.DocInfo, 0), nil
	}
	//4.执行文件导入
	resultList, err = docImportService.ImportDoc(ctx, task, docCheckList)
	return
}
