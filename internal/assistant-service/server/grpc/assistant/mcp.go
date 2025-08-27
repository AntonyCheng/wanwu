package assistant

import (
	"context"

	assistant_service "github.com/UnicomAI/wanwu/api/proto/assistant-service"
	errs "github.com/UnicomAI/wanwu/api/proto/err-code"
	"github.com/UnicomAI/wanwu/pkg/util"
	"google.golang.org/protobuf/types/known/emptypb"
)

// AssistantMCPCreate 添加mcp
func (s *Service) AssistantMCPCreate(ctx context.Context, req *assistant_service.AssistantMCPCreateReq) (*emptypb.Empty, error) {
	assistantId := util.MustU32(req.AssistantId)

	if status := s.cli.CreateAssistantMCP(ctx, assistantId, req.McpId, req.Identity.UserId, req.Identity.OrgId); status != nil {
		return nil, errStatus(errs.Code_AssistantMCPErr, status)
	}

	return &emptypb.Empty{}, nil
}

// AssistantMCPDelete 删除mcp
func (s *Service) AssistantMCPDelete(ctx context.Context, req *assistant_service.AssistantMCPDeleteReq) (*emptypb.Empty, error) {
	assistantId := util.MustU32(req.AssistantId)

	if status := s.cli.DeleteAssistantMCP(ctx, assistantId, req.McpId); status != nil {
		return nil, errStatus(errs.Code_AssistantMCPErr, status)
	}
	return &emptypb.Empty{}, nil
}

// AssistantMCPEnableSwitch mcp开关
func (s *Service) AssistantMCPEnableSwitch(ctx context.Context, req *assistant_service.AssistantMCPEnableSwitchReq) (*emptypb.Empty, error) {
	assistantId := util.MustU32(req.AssistantId)

	existingMCP, status := s.cli.GetAssistantMCP(ctx, assistantId, req.McpId)
	if status != nil {
		return nil, errStatus(errs.Code_AssistantMCPErr, status)
	}

	existingMCP.Enable = req.Enable
	if status = s.cli.UpdateAssistantMCP(ctx, existingMCP); status != nil {
		return nil, errStatus(errs.Code_AssistantMCPErr, status)
	}

	return &emptypb.Empty{}, nil
}
func (s *Service) AssistantMCPGetList(ctx context.Context, req *assistant_service.AssistantMCPGetListReq) (*assistant_service.AssistantMCPList, error) {
	assistantId := util.MustU32(req.AssistantId)

	mcpList, status := s.cli.GetAssistantMCPList(ctx, assistantId)
	if status != nil {
		return nil, errStatus(errs.Code_AssistantMCPErr, status)
	}

	assistantMCPInfos := make([]*assistant_service.AssistantMCPInfo, len(mcpList))
	for i, mcp := range mcpList {
		assistantMCPInfos[i] = &assistant_service.AssistantMCPInfo{
			Id:             mcp.ID,
			AssistantMcpId: mcp.AssistantId,
			McpId:          mcp.MCPId,
			Enable:         mcp.Enable,
			UserId:         mcp.UserId,
			OrgId:          mcp.OrgId,
			CreatedAt:      mcp.CreatedAt,
			UpdatedAt:      mcp.UpdatedAt,
		}
	}

	return &assistant_service.AssistantMCPList{
		AssistantMCPInfos: assistantMCPInfos,
	}, nil

}
