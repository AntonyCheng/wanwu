package client

import (
	"context"

	err_code "github.com/UnicomAI/wanwu/api/proto/err-code"
	rag_service "github.com/UnicomAI/wanwu/api/proto/rag-service"
	"github.com/UnicomAI/wanwu/internal/rag-service/client/model"
)

type IClient interface {
	DeleteRag(ctx context.Context, req *rag_service.RagDeleteReq) *err_code.Status
	GetRag(ctx context.Context, req *rag_service.RagDetailReq) (*rag_service.RagInfo, *err_code.Status)
	GetRagList(ctx context.Context, req *rag_service.RagListReq) (*rag_service.RagListResp, *err_code.Status)
	GetRagByIds(ctx context.Context, req *rag_service.GetRagByIdsReq) (*rag_service.AppBriefList, *err_code.Status)
	CreateRag(ctx context.Context, rag *model.RagInfo) *err_code.Status
	UpdateRag(ctx context.Context, rag *model.RagInfo) *err_code.Status
	UpdateRagConfig(ctx context.Context, rag *model.RagInfo) *err_code.Status
	FetchRagFirst(ctx context.Context, ragId string) (*model.RagInfo, *err_code.Status)
}
