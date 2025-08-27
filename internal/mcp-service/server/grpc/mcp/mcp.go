package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	errs "github.com/UnicomAI/wanwu/api/proto/err-code"
	mcp_service "github.com/UnicomAI/wanwu/api/proto/mcp-service"
	"github.com/UnicomAI/wanwu/internal/mcp-service/client/model"
	"github.com/UnicomAI/wanwu/internal/mcp-service/config"
	grpc_util "github.com/UnicomAI/wanwu/pkg/grpc-util"
	"github.com/UnicomAI/wanwu/pkg/util"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *Service) GetSquareMCP(ctx context.Context, req *mcp_service.GetSquareMCPReq) (*mcp_service.SquareMCPDetail, error) {
	mcpCfg, exist := config.Cfg().MCP(req.McpSquareId)
	if !exist {
		return nil, grpc_util.ErrorStatus(errs.Code_MCPGetSquareMCPErr)
	}
	hasCustom, err := s.cli.CheckMCPExist(ctx, req.OrgId, req.UserId, req.McpSquareId)
	if err != nil {
		return nil, errStatus(errs.Code_MCPGetSquareMCPErr, err)
	}
	return buildSquareMCPDetail(mcpCfg, hasCustom), nil
}

func (s *Service) GetSquareMCPList(ctx context.Context, req *mcp_service.GetSquareMCPListReq) (*mcp_service.SquareMCPList, error) {
	var resMcpSquareServers []*mcp_service.SquareMCPInfo
	for _, mcpCfg := range config.Cfg().Mcps {
		if req.Name != "" && !strings.Contains(mcpCfg.Name, req.Name) {
			continue
		}
		if !(req.Category == "" || req.Category == "all") && !strings.Contains(mcpCfg.Category, req.Category) {
			continue
		}
		resMcpSquareServers = append(resMcpSquareServers, buildSquareMCPInfo(*mcpCfg))
	}
	return &mcp_service.SquareMCPList{Infos: resMcpSquareServers}, nil
}

func (s *Service) CreateCustomMCP(ctx context.Context, req *mcp_service.CreateCustomMCPReq) (*emptypb.Empty, error) {
	if err := s.cli.CreateMCP(ctx, &model.MCPClient{
		OrgID:       req.OrgId,
		UserID:      req.UserId,
		McpSquareId: req.McpSquareId,
		Name:        req.Name,
		From:        req.From,
		Desc:        req.Desc,
		SseUrl:      req.SseUrl,
	}); err != nil {
		return nil, errStatus(errs.Code_MCPCreateCustomMCPErr, err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) GetCustomMCP(ctx context.Context, req *mcp_service.GetCustomMCPReq) (*mcp_service.CustomMCPDetail, error) {
	mcp, err := s.cli.GetMCP(ctx, util.MustU32(req.McpId))
	if err != nil {
		return nil, errStatus(errs.Code_MCPGetCustomMCPErr, err)
	}
	return buildCustomMCPDetail(mcp), nil
}

func (s *Service) DeleteCustomMCP(ctx context.Context, req *mcp_service.DeleteCustomMCPReq) (*emptypb.Empty, error) {
	if err := s.cli.DeleteMCP(ctx, util.MustU32(req.McpId)); err != nil {
		return nil, errStatus(errs.Code_MCPDeleteCustomMCPErr, err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) GetCustomMCPList(ctx context.Context, req *mcp_service.GetCustomMCPListReq) (*mcp_service.CustomMCPList, error) {
	mcps, err := s.cli.ListMCPs(ctx, req.OrgId, req.UserId, req.Name)
	if err != nil {
		return nil, errStatus(errs.Code_MCPGetCustomMCPListErr, err)
	}
	infos := make([]*mcp_service.CustomMCPInfo, 0, len(mcps))
	for _, mcp := range mcps {
		infos = append(infos, buildCustomMCPInfo(mcp))
	}
	return &mcp_service.CustomMCPList{Infos: infos}, nil
}
func (s *Service) GetCustomMCPByMCPIdList(ctx context.Context, req *mcp_service.GetCustomMCPByMCPIdListReq) (*mcp_service.CustomMCPList, error) {
	// 校验MCP ID列表是否为空
	if len(req.McpIdList) == 0 {
		return nil, errStatus(errs.Code_MCPGetCustomMCPListErr, toErrStatus("mcp_get_custom_tool_list_err", "mcp id list is empty"))
	}

	// 转换为uint32列表
	mcpIdList := make([]uint32, 0, len(req.McpIdList))
	for _, mcpId := range req.McpIdList {
		mcpIdList = append(mcpIdList, util.MustU32(mcpId))
	}

	mcps, err := s.cli.ListMCPsByMCPIdList(ctx, mcpIdList)
	if err != nil {
		return nil, errStatus(errs.Code_MCPGetCustomMCPListErr, err)
	}
	infos := make([]*mcp_service.CustomMCPInfo, 0, len(mcps))
	for _, mcp := range mcps {
		infos = append(infos, buildCustomMCPInfo(mcp))
	}
	return &mcp_service.CustomMCPList{Infos: infos}, nil
}

func (s *Service) GetMCPAvatar(ctx context.Context, req *mcp_service.GetMCPAvatarReq) (*mcp_service.MCPAvatar, error) {
	if req.AvatarPath == "" {
		return nil, errStatus(errs.Code_MCPGetMCPAvatarErr, toErrStatus("mcp_get_mcp_avatar_err", "avatar path is empty"))
	}
	data, err := os.ReadFile(filepath.Join(config.ConfigDir, req.AvatarPath))
	if err != nil {
		return nil, errStatus(errs.Code_MCPGetMCPAvatarErr, toErrStatus("mcp_get_mcp_avatar_err", err.Error()))
	}
	return &mcp_service.MCPAvatar{
		FileName: filepath.Base(req.AvatarPath),
		Data:     data,
	}, nil
}

// --- internal ---

func buildCustomMCPDetail(mcp *model.MCPClient) *mcp_service.CustomMCPDetail {
	ret := &mcp_service.CustomMCPDetail{
		McpId:  util.Int2Str(mcp.ID),
		SseUrl: mcp.SseUrl,
		Info: &mcp_service.SquareMCPInfo{
			McpSquareId: mcp.McpSquareId,
			AvatarPath:  config.MCPLogo,
			Name:        mcp.Name,
			Desc:        mcp.Desc,
			From:        mcp.From,
		},
	}
	if mcp.McpSquareId != "" {
		mcp, exist := config.Cfg().MCP(mcp.McpSquareId)
		if !exist {
			// 广场MCP不存在，则将McpSquareId置空
			ret.Info.McpSquareId = ""
		} else {
			ret.Info.AvatarPath = mcp.AvatarPath
			ret.Info.Category = mcp.Category
			ret.Intro = buildSquareMCPIntro(mcp)
		}
	}
	return ret
}

func buildCustomMCPInfo(mcp *model.MCPClient) *mcp_service.CustomMCPInfo {
	detail := buildCustomMCPDetail(mcp)
	return &mcp_service.CustomMCPInfo{
		McpId:  detail.McpId,
		SseUrl: detail.SseUrl,
		Info:   detail.Info,
	}
}

func buildSquareMCPDetail(mcpCfg config.McpConfig, hasCustom bool) *mcp_service.SquareMCPDetail {
	return &mcp_service.SquareMCPDetail{
		Info:  buildSquareMCPInfo(mcpCfg),
		Intro: buildSquareMCPIntro(mcpCfg),
		Tool: &mcp_service.MCPTools{
			SseUrl:    mcpCfg.SseUrl,
			HasCustom: hasCustom,
			Tools:     convertMCPTools(mcpCfg.Tools),
		},
	}
}

func buildSquareMCPInfo(mcpCfg config.McpConfig) *mcp_service.SquareMCPInfo {
	return &mcp_service.SquareMCPInfo{
		McpSquareId: mcpCfg.McpSquareId,
		AvatarPath:  mcpCfg.AvatarPath,
		Name:        mcpCfg.Name,
		Desc:        mcpCfg.Desc,
		From:        mcpCfg.From,
		Category:    mcpCfg.Category,
	}
}

func buildSquareMCPIntro(mcpCfg config.McpConfig) *mcp_service.SquareMCPIntro {
	return &mcp_service.SquareMCPIntro{
		Summary:  mcpCfg.Summary,
		Feature:  mcpCfg.Feature,
		Scenario: mcpCfg.Scenario,
		Manual:   mcpCfg.Manual,
		Detail:   mcpCfg.Detail,
	}
}

func convertMCPTools(tools []config.McpToolConfig) []*mcp_service.MCPTool {
	result := make([]*mcp_service.MCPTool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, &mcp_service.MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: convertMCPInputSchema(&tool.InputSchema),
		})
	}
	return result
}

func convertMCPInputSchema(schema *config.McpInputSchemaConfig) *mcp_service.MCPToolInputSchema {
	if schema == nil {
		return nil
	}

	properties := make(map[string]*mcp_service.MCPToolInputSchemaValue)
	for _, prop := range schema.Properties {
		properties[prop.Field] = &mcp_service.MCPToolInputSchemaValue{
			Type:        prop.Type,
			Description: prop.Description,
		}
	}

	return &mcp_service.MCPToolInputSchema{
		Type:       schema.Type,
		Required:   schema.Required,
		Properties: properties,
	}
}
