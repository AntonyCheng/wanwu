package v1

import (
	"github.com/UnicomAI/wanwu/internal/bff-service/model/request"
	gin_util "github.com/UnicomAI/wanwu/internal/bff-service/pkg/gin-util"
	"github.com/UnicomAI/wanwu/internal/bff-service/service"
	"github.com/gin-gonic/gin"
)

// ChatRag
//
//	@Tags		workspace.appspace.rag
//	@Summary	私域 RAG 问答
//	@Description
//	@Security	JWT
//	@Accept		json
//	@Produce	json
//	@Param		data	body		request.ChatRagRequest	true	"RAG问答请求参数"
//	@Success	200		{object}	response.Response
//	@Router		/rag/chat [post]
func ChatRag(ctx *gin.Context) {
	userId, orgId := getUserID(ctx), getOrgID(ctx)
	var req request.ChatRagRequest
	if !gin_util.Bind(ctx, &req) {
		return
	}
	if err := service.ChatRagStream(ctx, userId, orgId, req); err != nil {
		gin_util.Response(ctx, nil, err)
	}
}

// CreateRag
//
//	@Tags		workspace.appspace.rag
//	@Summary	创建RAG
//	@Description
//	@Security	JWT
//	@Accept		json
//	@Produce	json
//	@Param		data	body		request.AppBriefConfig	true	"创建RAG的请求参数"
//	@Success	200		{object}	response.Response{data=request.RagReq}
//	@Router		/appspace/rag [post]
func CreateRag(ctx *gin.Context) {
	userId, orgId := getUserID(ctx), getOrgID(ctx)
	var req request.AppBriefConfig
	if !gin_util.Bind(ctx, &req) {
		return
	}
	resp, err := service.CreateRag(ctx, userId, orgId, req)
	gin_util.Response(ctx, resp, err)
}

// UpdateRag
//
//	@Tags		workspace.appspace.rag
//	@Summary	更新RAG基本信息
//	@Description
//	@Security	JWT
//	@Accept		json
//	@Produce	json
//	@Param		data	body		request.RagBrief	true	"更新RAG基本信息的请求参数"
//	@Success	200		{object}	response.Response
//	@Router		/appspace/rag [put]
func UpdateRag(ctx *gin.Context) {
	var req request.RagBrief
	if !gin_util.Bind(ctx, &req) {
		return
	}
	err := service.UpdateRag(ctx, req)
	gin_util.Response(ctx, nil, err)
}

// UpdateRagConfig
//
//	@Tags		workspace.appspace.rag
//	@Summary	更新RAG配置信息
//	@Description
//	@Security	JWT
//	@Accept		json
//	@Produce	json
//	@Param		data	body		request.RagConfig	true	"更新RAG配置信息的请求参数"
//	@Success	200		{object}	response.Response
//	@Router		/appspace/rag/config [put]
func UpdateRagConfig(ctx *gin.Context) {
	var req request.RagConfig
	if !gin_util.Bind(ctx, &req) {
		return
	}
	err := service.UpdateRagConfig(ctx, req)
	gin_util.Response(ctx, nil, err)
}

// DeleteRag
//
//	@Tags		workspace.appspace.rag
//	@Summary	删除RAG
//	@Description
//	@Security	JWT
//	@Accept		json
//	@Produce	json
//	@Param		data	body		request.RagReq	true	"删除RAG的请求参数"
//	@Success	200		{object}	response.Response
//	@Router		/appspace/rag [delete]
func DeleteRag(ctx *gin.Context) {
	var req request.RagReq
	if !gin_util.Bind(ctx, &req) {
		return
	}
	err := service.DeleteRag(ctx, req)
	gin_util.Response(ctx, nil, err)
}

// GetRag
//
//	@Tags		workspace.appspace.rag
//	@Summary	获取RAG信息
//	@Description
//	@Security	JWT
//	@Accept		json
//	@Produce	json
//	@Param		data	query		request.RagReq	true	"获取RAG信息的请求参数"
//	@Success	200		{object}	response.Response{data=response.RagInfo}
//	@Router		/appspace/rag [get]
func GetRag(ctx *gin.Context) {
	var req request.RagReq
	if !gin_util.BindQuery(ctx, &req) {
		return
	}
	resp, err := service.GetRag(ctx, req)
	gin_util.Response(ctx, resp, err)
}
