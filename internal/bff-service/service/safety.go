package service

import (
	errs "github.com/UnicomAI/wanwu/api/proto/err-code"
	safety_service "github.com/UnicomAI/wanwu/api/proto/safety-service"
	"github.com/UnicomAI/wanwu/internal/bff-service/model/request"
	"github.com/UnicomAI/wanwu/internal/bff-service/model/response"
	grpc_util "github.com/UnicomAI/wanwu/pkg/grpc-util"
	"github.com/UnicomAI/wanwu/pkg/log"
	"github.com/UnicomAI/wanwu/pkg/minio"
	"github.com/UnicomAI/wanwu/pkg/util"
	"github.com/gin-gonic/gin"
)

const (
	AppSafetySensitiveUploadSingle = "single"
	AppSafetySensitiveUploadFile   = "file"
)

func CreateSensitiveWordTable(ctx *gin.Context, userId, orgId string, req *request.CreateSensitiveWordTableReq) (*response.CreateSensitiveWordTableResp, error) {
	resp, err := safety.CreateSensitiveWordTable(ctx, &safety_service.CreateSensitiveWordTableReq{
		UserId:    userId,
		OrgId:     orgId,
		TableName: req.TableName,
		Remark:    req.Remark,
	})
	if err != nil {
		return &response.CreateSensitiveWordTableResp{}, err
	}
	return &response.CreateSensitiveWordTableResp{TableId: resp.TableId}, err
}

func UpdateSensitiveWordTable(ctx *gin.Context, userId, orgId string, req *request.UpdateSensitiveWordTableReq) error {
	_, err := safety.UpdateSensitiveWordTable(ctx, &safety_service.UpdateSensitiveWordTableReq{
		OrgId:     orgId,
		UserId:    userId,
		TableId:   req.TableId,
		TableName: req.TableName,
		Remark:    req.Remark,
	})
	if err != nil {
		return err
	}
	return nil
}

func DeleteSensitiveWordTable(ctx *gin.Context, req *request.DeleteSensitiveWordTableReq) error {
	_, err := safety.DeleteSensitiveWordTable(ctx, &safety_service.DeleteSensitiveWordTableReq{
		TableId: req.TableId,
	})
	if err != nil {
		return err
	}
	return nil
}

func GetSensitiveWordTableList(ctx *gin.Context, userId, orgId string) (*response.ListResult, error) {
	listResult, err := safety.GetSensitiveWordTableList(ctx, &safety_service.GetSensitiveWordTableListReq{
		OrgId:  orgId,
		UserId: userId,
	})
	if err != nil {
		return nil, err
	}
	var retList = make([]*response.SensitiveWordTableDetail, 0)
	for _, detail := range listResult.List {
		retList = append(retList, &response.SensitiveWordTableDetail{
			TableId:   detail.TableId,
			TableName: detail.TableName,
			Remark:    detail.Remark,
			Reply:     detail.Reply,
			CreatedAt: util.Time2Str(detail.CreatedAt),
		})
	}
	return &response.ListResult{
		Total: listResult.Total,
		List:  retList,
	}, nil
}

func GetSensitiveVocabularyList(ctx *gin.Context, userId, orgId string, pageNo, pageSize int32, req *request.GetSensitiveVocabularyReq) (*response.PageResult, error) {
	listResult, err := safety.GetSensitiveVocabularyList(ctx, &safety_service.GetSensitiveVocabularyListReq{
		OrgId:    orgId,
		UserId:   userId,
		TableId:  req.TableId,
		PageSize: pageSize,
		PageNo:   pageNo,
	})
	if err != nil {
		return nil, err
	}
	var list = make([]*response.SensitiveWordVocabularyDetail, 0)
	if len(listResult.List) > 0 {
		for _, resp := range listResult.List {
			list = append(list, &response.SensitiveWordVocabularyDetail{
				WordId:        resp.WordId,
				Word:          resp.Word,
				SensitiveType: resp.SensitiveType,
			})
		}
	}
	return &response.PageResult{
		List:     list,
		Total:    listResult.Total,
		PageNo:   int(pageNo),
		PageSize: int(pageSize),
	}, nil
}

func UploadSensitiveVocabulary(ctx *gin.Context, userId, orgId string, req *request.UploadSensitiveVocabularyReq) error {
	var filePath string
	var err error
	if req.ImportType == AppSafetySensitiveUploadFile {
		filePath, err = minio.GetUploadFileWithExpire(ctx, req.FileName)
		if err != nil {
			log.Errorf("获取文件失败 %s，请稍后重试，%v", req.FileName, err)
			return grpc_util.ErrorStatus(errs.Code_AppSafetyImportUrlFailed)
		}
	}
	_, err = safety.UploadSensitiveVocabulary(ctx, &safety_service.UploadSensitiveVocabularyReq{
		OrgId:         orgId,
		UserId:        userId,
		FilePath:      filePath,
		TableId:       req.TableId,
		ImportType:    req.ImportType,
		Word:          req.Word,
		SensitiveType: req.SensitiveType,
	})
	if err != nil {
		return err
	}
	return nil
}

func DeleteSensitiveVocabulary(ctx *gin.Context, req *request.DeleteSensitiveVocabularyReq) error {
	_, err := safety.DeleteSensitiveVocabulary(ctx, &safety_service.DeleteSensitiveVocabularyReq{
		TableId: req.TableId,
		WordId:  req.WordId,
	})
	if err != nil {
		return err
	}
	return nil
}

func UpdateSensitiveWordTableReply(ctx *gin.Context, userId, orgId string, req *request.UpdateSensitiveWordTableReplyReq) error {
	_, err := safety.UpdateSensitiveWordTableReply(ctx, &safety_service.UpdateSensitiveWordTableReplyReq{
		OrgId:   orgId,
		UserId:  userId,
		TableId: req.TableId,
		Reply:   req.Reply,
	})
	if err != nil {
		return err
	}
	return nil
}
