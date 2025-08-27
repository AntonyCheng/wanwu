package assistant

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	net_url "net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	assistant_service "github.com/UnicomAI/wanwu/api/proto/assistant-service"
	"github.com/UnicomAI/wanwu/api/proto/common"
	errs "github.com/UnicomAI/wanwu/api/proto/err-code"
	knowledgebase_service "github.com/UnicomAI/wanwu/api/proto/knowledgebase-service"
	mcp_service "github.com/UnicomAI/wanwu/api/proto/mcp-service"
	"github.com/UnicomAI/wanwu/internal/assistant-service/client/model"
	"github.com/UnicomAI/wanwu/internal/assistant-service/config"
	"github.com/UnicomAI/wanwu/internal/assistant-service/pkg/util"
	"github.com/UnicomAI/wanwu/pkg/constant"
	"github.com/UnicomAI/wanwu/pkg/es"
	http_client "github.com/UnicomAI/wanwu/pkg/http-client"
	"github.com/UnicomAI/wanwu/pkg/log"
	mp "github.com/UnicomAI/wanwu/pkg/model-provider"
	pkgUtil "github.com/UnicomAI/wanwu/pkg/util"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ConversationCreate 创建对话
func (s *Service) ConversationCreate(ctx context.Context, req *assistant_service.ConversationCreateReq) (*assistant_service.ConversationCreateResp, error) {
	// 组装model参数
	assistantID, err := pkgUtil.U32(req.AssistantId)
	if err != nil {
		return nil, err
	}

	conversation := &model.Conversation{
		AssistantId: assistantID,
		Title:       req.Prompt, // 使用prompt作为初始标题
		UserId:      req.Identity.UserId,
		OrgId:       req.Identity.OrgId,
	}

	// 调用client方法创建对话
	if status := s.cli.CreateConversation(ctx, conversation); status != nil {
		return nil, errStatus(errs.Code_AssistantConversationErr, status)
	}

	return &assistant_service.ConversationCreateResp{
		ConversationId: strconv.FormatUint(uint64(conversation.ID), 10),
	}, nil
}

// ConversationDelete 删除对话
func (s *Service) ConversationDelete(ctx context.Context, req *assistant_service.ConversationDeleteReq) (*emptypb.Empty, error) {
	// 转换ID
	conversationID, err := strconv.ParseUint(req.ConversationId, 10, 32)
	if err != nil {
		return nil, err
	}

	// 调用client方法删除对话
	if status := s.cli.DeleteConversation(ctx, uint32(conversationID)); status != nil {
		return nil, errStatus(errs.Code_AssistantConversationErr, status)
	}

	return &emptypb.Empty{}, nil
}

// GetConversationList 对话列表
func (s *Service) GetConversationList(ctx context.Context, req *assistant_service.GetConversationListReq) (*assistant_service.GetConversationListResp, error) {
	// 计算offset
	offset := (req.PageNo - 1) * req.PageSize

	// 调用client方法获取对话列表
	conversations, total, status := s.cli.GetConversationList(ctx, req.AssistantId, req.Identity.UserId, req.Identity.OrgId, offset, req.PageSize)
	if status != nil {
		return nil, errStatus(errs.Code_AssistantConversationErr, status)
	}

	// 转换为响应格式
	var conversationInfos []*assistant_service.ConversationInfo
	for _, conversation := range conversations {
		conversationInfos = append(conversationInfos, &assistant_service.ConversationInfo{
			ConversationId: strconv.FormatUint(uint64(conversation.ID), 10),
			AssistantId:    strconv.FormatUint(uint64(conversation.AssistantId), 10),
			Title:          conversation.Title,
			CreatTime:      conversation.CreatedAt,
		})
	}

	return &assistant_service.GetConversationListResp{
		Data:     conversationInfos,
		Total:    total,
		PageSize: req.PageSize,
		PageNo:   req.PageNo,
	}, nil
}

// GetConversationDetailList 对话详情历史列表
func (s *Service) GetConversationDetailList(ctx context.Context, req *assistant_service.GetConversationDetailListReq) (*assistant_service.GetConversationDetailListResp, error) {
	// 计算分页参数
	from := (req.PageNo - 1) * req.PageSize
	size := int(req.PageSize)

	// 组装查询条件
	fieldConditions := map[string]interface{}{
		"conversationId": req.ConversationId,
		"userId":         req.Identity.UserId,
		"orgId":          req.Identity.OrgId,
	}

	// 使用通配符查询所有对话详情索引
	indexPattern := "conversation_detail_infos_*"

	// 从ES查询数据
	documents, total, err := es.Assistant().SearchByFields(ctx, indexPattern, fieldConditions, int(from), size)
	if err != nil {
		log.Errorf("从ES查询对话详情失败，conversationId: %s, userId: %s, error: %v", req.ConversationId, req.Identity.UserId, err)
		return nil, fmt.Errorf("查询对话详情失败: %v", err)
	}

	// 转换查询结果为响应格式
	var conversationDetails []*assistant_service.ConversionDetailInfo
	for _, doc := range documents {
		var detail model.ConversationDetails
		if err := json.Unmarshal(doc, &detail); err != nil {
			log.Warnf("解析ES文档失败: %v", err)
			continue
		}

		conversationDetails = append(conversationDetails, &assistant_service.ConversionDetailInfo{
			Id:              detail.Id,
			AssistantId:     detail.AssistantId,
			ConversationId:  detail.ConversationId,
			Prompt:          detail.Prompt,
			SysPrompt:       detail.SysPrompt,
			Response:        detail.Response,
			SearchList:      detail.SearchList,
			QaType:          detail.QaType,
			CreatedBy:       detail.UserId, // 使用CreatedBy字段映射UserId
			CreatedAt:       detail.CreatedAt,
			UpdatedAt:       detail.UpdatedAt,
			RequestFileUrls: []string{detail.FileUrl},
			FileSize:        detail.FileSize,
			FileName:        detail.FileName,
		})
	}

	log.Infof("成功从ES查询对话详情，conversationId: %s, userId: %s, 总数: %d, 返回: %d",
		req.ConversationId, req.Identity.UserId, total, len(conversationDetails))

	return &assistant_service.GetConversationDetailListResp{
		Data:     conversationDetails,
		Total:    total,
		PageSize: req.PageSize,
		PageNo:   req.PageNo,
	}, nil
}

// AssistantConversionStream 智能体流式对话
func (s *Service) AssistantConversionStream(req *assistant_service.AssistantConversionStreamReq, stream assistant_service.AssistantService_AssistantConversionStreamServer) error {
	ctx := stream.Context()
	reqUserId := req.Identity.UserId
	log.Debugf("Assistant服务开始智能体流式对话，assistantId: %s, userId: %s, orgId: %s, conversationId: %s, fileInfo: %+v, trial: %v, prompt: %s",
		req.AssistantId, reqUserId, req.Identity.OrgId, req.ConversationId, req.FileInfo, req.Trial, req.Prompt)

	// 用于跟踪流式响应状态的变量
	var fullResponse strings.Builder
	var searchList string
	var hasReadFirstMessage bool
	var streamStarted bool
	var conversationSaved bool // 标记是否已经保存过对话

	// 使用defer统一处理上下文取消的情况
	defer func() {
		// 只有在上下文被手动取消且还未保存过对话时，才保存"已被终止"消息
		if ctx.Err() != nil && !req.Trial && !conversationSaved {
			if !streamStarted {
				// 流式响应还未开始，保存基本终止消息
				saveConversation(ctx, req, "本次回答已被终止", "")
			} else {
				// 流式响应已开始
				if !hasReadFirstMessage {
					// 如果还没有读取到第一条消息，保存终止消息
					saveConversation(ctx, req, "本次回答已被终止", searchList)
				} else {
					// 如果已经读取到消息，保存已经收到的消息
					saveConversation(ctx, req, fullResponse.String()+"\n本次回答已被终止", searchList)
				}
			}
		}
	}()

	// 根据智能体id查询智能体信息
	assistantID, err := strconv.ParseUint(req.AssistantId, 10, 32)
	if err != nil {
		log.Errorf("Assistant服务智能体ID转换失败，assistantId: %s, error: %v", req.AssistantId, err)
		return err
	}

	assistant, status := s.cli.GetAssistant(ctx, uint32(assistantID))
	if status != nil {
		log.Errorf("Assistant服务获取智能体信息失败，assistantId: %s, error: %v", req.AssistantId, status)
		SSEError(stream, "智能体信息获取失败")
		return errStatus(errs.Code_AssistantConversationErr, status)
	}

	log.Debugf("Assistant服务获取到智能体信息，assistantId: %s, 名称: %s, Scope: %d, userId: %s, orgId: %s",
		req.AssistantId, assistant.Name, assistant.Scope, assistant.UserId, assistant.OrgId)

	// 获取Assistant配置
	assistantConfig := config.Cfg().Assistant
	if assistantConfig.SseUrl == "" {
		log.Errorf("Assistant服务SSE URL配置为空，assistantId: %s", req.AssistantId)
		SSEError(stream, "智能体配置错误")
		return fmt.Errorf("智能体配置错误")
	}

	// 组装智能体能力接口请求体
	sseReq := &config.AgentSSERequest{
		Input:        req.Prompt,
		Stream:       true,
		AutoCitation: true,
	}

	if assistant.Instructions != "" {
		sseReq.SystemRole = assistant.Instructions
	}

	if req.FileInfo.FileUrl != "" {
		sseReq.UploadFileUrl = req.FileInfo.FileUrl
		sseReq.FileName = req.FileInfo.FileName
	}

	// 模型参数配置
	modelConfig, err := s.setModelConfigParams(sseReq, assistant)
	if err != nil {
		SSEError(stream, "智能体模型配置解析失败")
		return err
	}

	// 知识库参数配置
	if err := s.setKnowledgebaseParams(ctx, sseReq, req, assistant, modelConfig); err != nil {
		SSEError(stream, "智能体知识库配置解析失败")
		return err
	}

	// plugin参数配置
	if err := s.setCustomAndWorkflowParams(ctx, sseReq, req.AssistantId, req.AccessedWorkFlowIds); err != nil {
		SSEError(stream, "智能体plugin配置错误")
		return err
	}

	// 在线搜索参数配置
	if err := s.setOnlineSearchParams(sseReq, assistant); err != nil {
		SSEError(stream, "智能体在线搜索配置解析失败")
		return err
	}

	// MCP 信息参数配置
	if err := s.setMCPParams(ctx, sseReq, assistant); err != nil {
		SSEError(stream, "智能体MCP配置解析失败")
		return err
	}

	// 历史聊天记录配置
	if !req.Trial && req.ConversationId != "" {
		s.setHistoryParams(ctx, sseReq, req)
	}

	// 底层智能体能力接口请求体
	var requestBody map[string]interface{}
	reqBytes, err := json.Marshal(sseReq)
	if err != nil {
		log.Errorf("Assistant服务序列化请求体失败，assistantId: %s, error: %v", req.AssistantId, err)
		SSEError(stream, "请求参数错误")
		return err
	}
	if err := json.Unmarshal(reqBytes, &requestBody); err != nil {
		log.Errorf("Assistant服务反序列化请求体到map失败，assistantId: %s, error: %v", req.AssistantId, err)
		SSEError(stream, "请求参数错误")
		return err
	}

	// 合并动态模型参数
	if sseReq.ModelParams != nil {
		requestBody = mergeMaps(requestBody, sseReq.ModelParams)
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Errorf("Assistant服务序列化最终请求体失败，assistantId: %s, error: %v", req.AssistantId, err)
		SSEError(stream, "请求参数错误")
		return err
	}

	timeout := 300 * time.Second
	startTime := time.Now()
	id := uuid.New().String()

	// 公开的智能体，xuid使用智能体创建者用户信息
	xuid := reqUserId
	if req.AppPublishType == constant.AppPublishPublic {
		xuid = assistant.UserId
		log.Debugf("Assistant服务公开智能体，使用创建者信息，assistantId: %s, userId: %s", req.AssistantId, assistant.UserId)
	}

	log.Infof("Assistant服务开始调用HttpRequestLlmStream，uuid: %s, assistantId: %s, url: %s, userId: %s, timeout: %v, body: %s",
		id, req.AssistantId, assistantConfig.SseUrl, reqUserId, timeout, string(requestBodyBytes))
	sseResp, err := HttpRequestLlmStream(ctx, assistantConfig.SseUrl, reqUserId, xuid, bytes.NewReader(requestBodyBytes), timeout)
	if err != nil {
		log.Errorf("Assistant服务调用智能体能力接口失败，assistantId: %s, uuid: %s, error: %v", req.AssistantId, id, err)
		SSEError(stream, "智能体服务异常")
		return err
	}
	defer sseResp.Body.Close()
	log.Infof("Assistant服务成功连接智能体能力接口，uuid: %s, assistantId: %s, statusCode: %d, time: %v毫秒", id, req.AssistantId, sseResp.StatusCode, time.Since(startTime).Milliseconds())

	// SSE 请求返回Code大于400
	if sseResp.StatusCode > http.StatusBadRequest {
		log.Errorf("Assistant服务智能体能力接口返回错误状态码，assistantId: %s, statusCode: %d", req.AssistantId, sseResp.StatusCode)
		SSEError(stream, "智能体服务异常")
		return fmt.Errorf("智能体服务返回错误状态码: %d", sseResp.StatusCode)
	}

	// 读取智能体接口返回，并写入流式响应
	reader := bufio.NewReader(sseResp.Body)
	lineCount := 0

	// 标记流式响应已开始
	streamStarted = true
	searchListExtracted := false

	for {

		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			log.Errorf("Assistant服务读取流式响应失败，assistantId: %s, error: %v, 已处理行数: %d", req.AssistantId, err, lineCount)

			// 检查是否是上下文取消导致的错误
			if ctx.Err() != nil {
				// 用户手动取消请求，让defer函数处理"已被终止"消息，这里不保存
				log.Debugf("Assistant服务检测到上下文取消，assistantId: %s", req.AssistantId)
			} else {
				// 真正的SSE读取错误，保存"已中断"消息
				if !req.Trial {
					if !hasReadFirstMessage {
						// 如果还没有读取到第一条消息，保存中断消息
						saveConversation(ctx, req, "本次回答已中断", searchList)
					} else {
						// 如果已经读取到消息，保存已经收到的消息
						saveConversation(ctx, req, fullResponse.String()+"\n本次回答已中断", searchList)
					}
					conversationSaved = true // 标记已保存，避免defer中重复保存
				}
				SSEError(stream, "本次回答已中断")
			}
			return err
		}

		strLine := string(line)
		lineCount++

		if len(strLine) >= 5 && strLine[:5] == "data:" {

			jsonStrData := strLine[5:]

			// 解析流式数据，提取response字段和search_list
			var streamData map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStrData), &streamData); err == nil {
				if response, ok := streamData["response"].(string); ok && response != "" {
					fullResponse.WriteString(response)
				}

				// 提取第一个search_list
				if !searchListExtracted {
					if searchListData, ok := streamData["search_list"]; ok {
						searchListBytes, err := json.Marshal(searchListData)
						if err == nil {
							searchList = string(searchListBytes)
							searchListExtracted = true
							log.Debugf("Assistant服务提取到search_list，assistantId: %s, searchList: %s", req.AssistantId, searchList)
						}
					}
				}
			}

			if err := stream.Send(&assistant_service.AssistantConversionStreamResp{
				Content: jsonStrData,
			}); err != nil {
				log.Errorf("Assistant服务发送流式响应失败，assistantId: %s, error: %v", req.AssistantId, err)
				return err
			}

			// 标记已读取到并返回了第一条有效消息
			if !hasReadFirstMessage {
				hasReadFirstMessage = true
			}

		}

		if err != nil && (err == io.ErrUnexpectedEOF || err == io.EOF) {
			log.Debugf("Assistant服务流式响应结束，assistantId: %s, 总处理行数: %d", req.AssistantId, lineCount)
			break
		}
	}

	// 问答调试不保存
	if !req.Trial {
		saveConversation(ctx, req, fullResponse.String(), searchList)
		conversationSaved = true // 标记已保存
	}

	return nil
}

// 设置模型配置参数
func (s *Service) setModelConfigParams(sseReq *config.AgentSSERequest, assistant *model.Assistant) (*common.AppModelConfig, error) {
	if assistant.ModelConfig == "" {
		log.Warnf("Assistant服务智能体模型配置为空，assistantId: %s", assistant.ID)
		return nil, nil
	}

	log.Debugf("Assistant服务解析模型配置，assistantId: %s, modelConfig: %s", assistant.ID, assistant.ModelConfig)
	modelConfig := &common.AppModelConfig{}
	if err := json.Unmarshal([]byte(assistant.ModelConfig), modelConfig); err != nil {
		return nil, fmt.Errorf("Assistant服务解析智能体模型配置失败，assistantId: %d, error: %v, modelConfigRaw: %s", assistant.ID, err, assistant.ModelConfig)
	}

	log.Debugf("Assistant服务成功解析智能体模型配置，assistantId: %s, provider: %s, model: %s, modelId: %s, modelType: %s",
		assistant.ID, modelConfig.Provider, modelConfig.Model, modelConfig.ModelId, modelConfig.ModelType)

	modelEndpoint := mp.ToModelEndpoint(modelConfig.ModelId, modelConfig.Model)
	log.Debugf("Assistant服务生成模型端点，assistantId: %s, modelEndpoint: %+v", assistant.ID, modelEndpoint)
	sseReq.Model = modelEndpoint["model"].(string)
	sseReq.ModelUrl = modelEndpoint["model_url"].(string)

	_, modelParams, _ := mp.ToModelParams(modelConfig.Provider, modelConfig.ModelType, modelConfig.Config)
	log.Debugf("Assistant服务生成模型参数，assistantId: %s, modelParams: %+v", assistant.ID, modelParams)
	if modelParams != nil {
		sseReq.ModelParams = modelParams
	}

	return modelConfig, nil
}

// 设置知识库参数
func (s *Service) setKnowledgebaseParams(ctx context.Context, sseReq *config.AgentSSERequest, req *assistant_service.AssistantConversionStreamReq, assistant *model.Assistant, modelConfig *common.AppModelConfig) error {
	knowledgebaseConfig := &RAGKnowledgeBaseConfig{}
	if assistant.KnowledgebaseConfig == "" {
		return nil
	}

	if err := json.Unmarshal([]byte(assistant.KnowledgebaseConfig), knowledgebaseConfig); err != nil {
		log.Errorf("Assistant服务解析智能体知识库配置失败，assistantId: %s, error: %v, knowledgebaseConfigRaw: %s", req.AssistantId, err, assistant.KnowledgebaseConfig)
		return err
	}
	log.Debugf("Assistant服务解析知识库成功，knowledgebaseConfig: %+v", knowledgebaseConfig)

	if len(knowledgebaseConfig.KnowledgeBaseIds) > 0 {
		rerankEndpoint, err := buildRerank(req, knowledgebaseConfig, assistant)
		if err != nil {
			return err
		}
		knowledgeInfoList, err := Knowledge.SelectKnowledgeDetailByIdList(ctx, &knowledgebase_service.KnowledgeDetailSelectListReq{
			KnowledgeIds: knowledgebaseConfig.KnowledgeBaseIds,
		})
		if err != nil {
			log.Errorf("Assistant服务获取知识库详情失败, err: %v", err)
			return err
		}
		log.Infof("knowledgeInfoList = %+v", knowledgeInfoList)

		var knowNames []string
		for _, v := range knowledgeInfoList.List {
			knowNames = append(knowNames, v.Name)
		}

		sseReq.KnParams = &config.KnParams{
			KnowledgeBase:  knowNames,
			RerankId:       rerankEndpoint["model_id"],
			Model:          rerankEndpoint["model"],
			ModelUrl:       rerankEndpoint["model_url"],
			RerankMod:      buildRerankMod(knowledgebaseConfig.PriorityMatch),
			RetrieveMethod: buildRetrieveMethod(knowledgebaseConfig.MatchType),
			Weights:        buildWeight(knowledgebaseConfig),
			MaxHistory:     int(knowledgebaseConfig.MaxHistory),
			Threshold:      knowledgebaseConfig.Threshold,
			TopK:           int(knowledgebaseConfig.TopK),
			RewriteQuery:   true,
		}
		sseReq.UseKnow = true
		if modelConfig != nil {
			sseReq.ModelId = modelConfig.ModelId
		}
	}
	return nil
}

// 设置plugin参数：自定义工具和工作流
func (s *Service) setCustomAndWorkflowParams(ctx context.Context, sseReq *config.AgentSSERequest, assistantId string, accessedWorkFlowIds []string) error {
	customPluginList, err := buildCustomToolListAlgParam(ctx, s, assistantId)
	if err != nil {
		return fmt.Errorf("智能体custom配置错误: %w", err)
	}

	workflowPluginList, err := buildWorkflowPluginListAlgParam(ctx, s, assistantId, accessedWorkFlowIds)
	if err != nil {
		return fmt.Errorf("智能体workflow配置错误: %w", err)
	}

	log.Debugf("智能体workflow配置，assistantId: %s, workflowPluginList: %s", assistantId, workflowPluginList)
	allPlugin := append(customPluginList, workflowPluginList...)
	sseReq.PluginList = allPlugin
	log.Debugf("智能体custom_plugin_list，assistantId: %s, custom_plugin_list: %s", assistantId, allPlugin)
	return nil
}

// 设置在线搜索参数
func (s *Service) setOnlineSearchParams(sseReq *config.AgentSSERequest, assistant *model.Assistant) error {
	if assistant.OnlineSearchConfig == "" {
		return nil
	}

	onlineSearchConfig := &AppOnlineSearchConfig{}
	log.Debugf("Assistant服务解析智能体在线搜索配置，assistantId: %s, onlineSearchConfig: %+v", assistant.ID, assistant.OnlineSearchConfig)
	if err := json.Unmarshal([]byte(assistant.OnlineSearchConfig), onlineSearchConfig); err != nil {
		return fmt.Errorf("Assistant服务解析智能体在线搜索配置失败，assistantId: %d, error: %v, onlineSearchConfigRaw: %s", assistant.ID, err, assistant.OnlineSearchConfig)
	}
	log.Debugf("Assistant服务解析智能体在线搜索配置，assistantId: %s, onlineSearchConfig: %+v", assistant.ID, onlineSearchConfig)

	if onlineSearchConfig.Enable && onlineSearchConfig.SearchUrl != "" && onlineSearchConfig.SearchKey != "" {
		sseReq.SearchUrl = onlineSearchConfig.SearchUrl
		sseReq.SearchKey = onlineSearchConfig.SearchKey
		sseReq.SearchRerankId = onlineSearchConfig.SearchRerankId
		sseReq.UseSearch = true
		log.Debugf("Assistant服务添加在线搜索配置到请求参数，assistantId: %s, search_url: %s, search_key: %s, use_search: %v", assistant.ID, onlineSearchConfig.SearchUrl, onlineSearchConfig.SearchKey, onlineSearchConfig.Enable)
	}
	return nil
}

// 设置MCP参数
func (s *Service) setMCPParams(ctx context.Context, sseReq *config.AgentSSERequest, assistant *model.Assistant) error {
	mcpInfos, err := s.cli.GetAssistantMCPList(ctx, assistant.ID)
	if err != nil {
		return fmt.Errorf("Assistant服务获取MCP信息失败，assistantId: %d, error: %v", assistant.ID, err)
	}

	mcpTools := make(map[string]config.MCPToolInfo)
	for _, m := range mcpInfos {
		if !m.Enable {
			continue
		}
		mcpCustom, err := MCP.GetCustomMCP(ctx, &mcp_service.GetCustomMCPReq{
			McpId: m.MCPId,
		})
		if err != nil {
			log.Errorf("Assistant服务获取MCP Custom信息失败，assistantId: %d, error: %v", assistant.ID, err)
			// 单个MCP获取失败不影响整体流程
			continue
		}
		mcpTools[mcpCustom.Info.Name] = config.MCPToolInfo{
			URL:       mcpCustom.SseUrl,
			Transport: "sse",
		}
	}
	sseReq.McpTools = mcpTools
	return nil
}

// 设置历史记录参数
func (s *Service) setHistoryParams(ctx context.Context, sseReq *config.AgentSSERequest, req *assistant_service.AssistantConversionStreamReq) {
	fieldConditions := map[string]interface{}{
		"conversationId": req.ConversationId,
		"userId":         req.Identity.UserId,
		"orgId":          req.Identity.OrgId,
	}
	indexPattern := "conversation_detail_infos_*"

	documents, _, err := es.Assistant().SearchByFields(ctx, indexPattern, fieldConditions, 0, 1000)
	if err != nil {
		log.Warnf("Assistant服务查询历史聊天记录失败，conversationId: %s, userId: %s, error: %v", req.ConversationId, req.Identity.UserId, err)
		return
	}

	var historyList []config.AssistantConversionHistory
	for _, doc := range documents {
		var detail model.ConversationDetails
		if err := json.Unmarshal(doc, &detail); err != nil {
			log.Warnf("Assistant服务解析ES历史聊天记录失败: %v", err)
			continue
		}
		history := config.AssistantConversionHistory{
			Query:         detail.Prompt,
			UploadFileUrl: detail.FileUrl,
			Response:      detail.Response,
		}
		historyList = append(historyList, history)
	}

	if len(historyList) > 0 {
		sseReq.History = historyList
		log.Debugf("Assistant服务添加历史聊天记录到请求参数，conversationId: %s, 历史记录数: %d", req.ConversationId, len(historyList))
	}
}

func buildRerank(req *assistant_service.AssistantConversionStreamReq, knowledgebaseConfig *RAGKnowledgeBaseConfig, assistant *model.Assistant) (map[string]interface{}, error) {
	rerankModel := map[string]interface{}{}

	log.Debugf("Assistant服务解析知识库rerank配置，assistantId: %s", req.AssistantId)
	if knowledgebaseConfig.PriorityMatch != 1 {
		rerankConfig := &common.AppModelConfig{}
		if assistant.RerankConfig != "" {
			if err := json.Unmarshal([]byte(assistant.RerankConfig), rerankConfig); err != nil {
				log.Errorf("Assistant服务解析智能体rerank配置失败，assistantId: %s, error: %v, rerankConfigRaw: %s", req.AssistantId, err, assistant.RerankConfig)
				return nil, err
			}
			if rerankConfig.Model == "" || rerankConfig.ModelId == "" {
				log.Errorf("Assistant服务缺少rerank配置，assistantId: %s", req.AssistantId)
				return nil, fmt.Errorf("智能体缺少rerank配置")
			}
		}
		rerankModel = mp.ToModelEndpoint(rerankConfig.ModelId, rerankConfig.Model)
	}
	return rerankModel, nil
}

// 使用独立上下文保存对话的辅助函数
func saveConversation(originalCtx context.Context, req *assistant_service.AssistantConversionStreamReq, response, searchList string) {
	// 如果原始上下文已取消，创建一个新的独立上下文
	if originalCtx.Err() != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := saveConversationDetailToES(ctx, req, response, searchList); err != nil {
			log.Errorf("保存聊天记录到ES失败，assistantId: %s, conversationId: %s, error: %v",
				req.AssistantId, req.ConversationId, err)
		}
		return
	}

	// 原始上下文未取消时，继续使用它
	if err := saveConversationDetailToES(originalCtx, req, response, searchList); err != nil {
		log.Errorf("保存聊天记录到ES失败，assistantId: %s, conversationId: %s, error: %v",
			req.AssistantId, req.ConversationId, err)
	}
}

// buildRetrieveMethod 构造检索方式
func buildRetrieveMethod(matchType string) string {
	switch matchType {
	case "vector":
		return "semantic_search"
	case "text":
		return "full_text_search"
	case "mix":
		return "hybrid_search"
	}
	return ""
}

// buildRerankMod 构造重排序模式
func buildRerankMod(priorityType int32) string {
	if priorityType == 1 {
		return "weighted_score"
	}
	return "rerank_model"
}

// buildWeight 构造权重信息
func buildWeight(knowConfig *RAGKnowledgeBaseConfig) []float64 {
	if knowConfig.PriorityMatch != 1 {
		return nil
	}
	return []float64{float64(knowConfig.SemanticsPriority), float64(knowConfig.KeywordPriority)}
}

type AppKnowledgebaseConfig struct {
	Knowledgebases []AppKnowledgeBase     `json:"knowledgebases"`
	Config         AppKnowledgebaseParams `json:"config"`
}

type AppKnowledgeBase struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AppKnowledgebaseParams struct {
	MaxHistory int32   `json:"maxHistory"` // 最长上下文
	Threshold  float32 `json:"threshold"`  // 过滤阈值
	TopK       int32   `json:"topK"`       // 知识条数

	MatchType         string  `json:"matchType"`         //matchType：vector（向量检索）、text（文本检索）、mix（混合检索：向量+文本）
	PriorityMatch     int32   `json:"priorityMatch"`     // 权重匹配，只有在混合检索模式下，选择权重设置后，这个才设置为1
	SemanticsPriority float32 `json:"semanticsPriority"` // 语义权重
	KeywordPriority   float32 `json:"keywordPriority"`   // 关键词权重
}

// RAGKnowledgeBaseConfig 知识库配置结构体
type RAGKnowledgeBaseConfig struct {
	KnowledgeBaseIds  []string `json:"knowledgeBaseIds"`  // 知识库信息
	MaxHistory        int32    `json:"maxHistory"`        // 最长上下文
	Threshold         float32  `json:"threshold"`         // 过滤阈值
	TopK              int32    `json:"topK"`              // topK
	MatchType         string   `json:"matchType"`         // 检索类型：vector（向量检索）、text（文本检索）、mix（混合检索）
	KeywordPriority   float32  `json:"keywordPriority"`   // 关键词权重
	PriorityMatch     int32    `json:"priorityMatch"`     // 权重匹配，仅混合检索模式下有效，1 表示启用
	SemanticsPriority float32  `json:"semanticsPriority"` // 语义权重
}

type AppOnlineSearchConfig struct {
	SearchUrl      string `json:"searchUrl"`
	SearchKey      string `json:"searchKey"`
	SearchRerankId string `json:"SearchRerankId"`
	Enable         bool   `json:"enable"`
}

type WeightParams struct {
	VectorWeight float32 `json:"vector_weight"` //语义权重
	TextWeight   float32 `json:"text_weight"`   //关键字权重
}

func mergeMaps(map1, map2 map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range map1 {
		result[k] = v
	}
	for k, v := range map2 {
		result[k] = v // 若 key 重复，map2 的值覆盖 map1
	}
	return result
}

func buildWorkflowPluginListAlgParam(ctx context.Context, s *Service, assistantId string, accessedWorkFlowIds []string) (pluginList []config.PluginListAlgRequest, err error) {
	workflows, status := s.cli.GetAssistantWorkflowsByAssistantID(ctx, pkgUtil.MustU32(assistantId))
	if status != nil {
		return nil, errStatus(errs.Code_AssistantConversationErr, status)
	}
	// workflow ids
	var workflowIDs []string
	for _, workflow := range workflows {
		if !workflow.Enable {
			continue
		}
		if !slices.Contains(accessedWorkFlowIds, workflow.WorkflowId) {
			continue
		}
		workflowIDs = append(workflowIDs, workflow.WorkflowId)
	}
	if len(workflowIDs) == 0 {
		return nil, nil
	}
	// workflow schemas
	url, _ := net_url.JoinPath(config.Cfg().Workflow.Endpoint, config.Cfg().Workflow.ListSchemaUri)
	reqBody, _ := json.Marshal(map[string]interface{}{
		"workflow_ids": workflowIDs,
	})
	result, err := http_client.Workflow().PostJson(ctx, &http_client.HttpRequestParams{
		Url:        url,
		Body:       reqBody,
		Timeout:    time.Minute,
		MonitorKey: "workflow_schema",
		LogLevel:   http_client.LogAll,
	})
	if err != nil {
		return nil, err
	}
	var schemas []map[string]interface{}
	if err = json.Unmarshal(result, &schemas); err != nil {
		return nil, err
	}
	for _, schema := range schemas {
		schemaByte, err := json.Marshal(schema)
		if err != nil {
			return nil, err
		}
		//校验schema
		if _, err := util.ValidateOpenAPISchema(string(schemaByte)); err != nil {
			return nil, err
		}
		pluginList = append(pluginList, config.PluginListAlgRequest{APISchema: schema})
	}
	log.Infof("Assistant服务查询到workflow，assistantId: %s, workflowList: %v", assistantId, pluginList)
	return pluginList, nil
}

func buildCustomToolListAlgParam(ctx context.Context, s *Service, assistantId string) (pluginList []config.PluginListAlgRequest, err error) {
	pluginList = []config.PluginListAlgRequest{}
	// 获取自定义工具列表
	assistantIdConv := pkgUtil.MustU32(assistantId)
	resp, status := s.cli.GetAssistantCustomList(ctx, assistantIdConv)
	if status != nil {
		return pluginList, errStatus(errs.Code_AssistantConversationErr, status)
	}
	for _, assistantCustomTool := range resp {
		if !assistantCustomTool.Enable {
			continue
		}
		tmp := config.PluginListAlgRequest{}
		// 获取自定义工具详情
		info, err := MCP.GetCustomToolInfo(ctx, &mcp_service.GetCustomToolInfoReq{
			CustomToolId: assistantCustomTool.CustomId,
		})
		if err != nil {
			//return pluginList, err
			log.Infof("Assistant服务获取CustomTool信息失败，assistantId: %s,error: %v", assistantId, err)
			continue
		}
		schema, err := util.ValidateOpenAPISchema(info.Schema)
		if err != nil {
			return pluginList, err
		}
		// 将*openapi3.T转换为map[string]interface{}
		bytes, err := json.Marshal(schema)
		if err != nil {
			return pluginList, err
		}
		err = json.Unmarshal(bytes, &tmp.APISchema)
		if err != nil {
			return pluginList, err
		}

		if info.ApiAuth.Type == "apiKey" {
			apiAuth := config.APIAuth{
				Type:  "apiKey",
				In:    "query",
				Name:  info.ApiAuth.CustomHeaderName,
				Value: info.ApiAuth.ApiKey,
			}
			tmp.APIAuth = &apiAuth
		}
		//TODO 适配 assistantActionModel.Type ==None情况
		pluginList = append(pluginList, tmp)
	}

	return pluginList, nil
}

// SSEError 发送SSE错误响应
func SSEError(stream assistant_service.AssistantService_AssistantConversionStreamServer, message string) {
	log.Errorf("SSE错误: %s", message)
	// 通过流式响应发送错误信息
	if stream != nil {
		errorResponse := fmt.Sprintf("error:%s", message)
		if err := stream.Send(&assistant_service.AssistantConversionStreamResp{
			Content: errorResponse,
		}); err != nil {
			log.Errorf("发送SSE错误响应失败: %v", err)
		} else {
			log.Infof("成功发送SSE错误响应: %s", message)
		}
	} else {
		log.Warnf("stream为nil，无法发送SSE错误响应: %s", message)
	}
}

func HttpRequestLlmStream(ctx context.Context, url, userId, xuid string, body io.Reader, timeout time.Duration) (*http.Response, error) {
	requestCtx, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		log.Errorf("HttpRequestLlmStream创建HTTP请求失败，url: %s, userId: %s, error: %v", url, userId, err)
		return nil, err
	}

	// 设置请求头
	requestCtx.Header.Set("Content-Type", "application/json")
	requestCtx.Header.Set("X-Uid", xuid)

	log.Debugf("HttpRequestLlmStream请求详情，url: %s, userId: %s, method: %s, headers: %+v",
		url, userId, requestCtx.Method, requestCtx.Header)

	// 创建客户端并发送请求
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	response, err := client.Do(requestCtx)
	if err != nil {
		log.Errorf("HttpRequestLlmStream发送HTTP请求失败，url: %s, userId: %s, error: %v", url, userId, err)
		return nil, err
	}

	log.Debugf("HttpRequestLlmStream收到HTTP响应，url: %s, userId: %s, statusCode: %d, responseHeaders: %+v",
		url, userId, response.StatusCode, response.Header)

	return response, err
}

// saveConversationDetailToES 保存聊天记录到ES
func saveConversationDetailToES(ctx context.Context, req *assistant_service.AssistantConversionStreamReq, response, searchList string) error {
	// 根据当前时间生成索引名称，格式为conversation_detail_infos_YYYYMM
	now := time.Now()
	indexName := fmt.Sprintf("conversation_detail_infos_%d%02d", now.Year(), now.Month())

	// 组装ConversationDetails数据
	nowMilli := now.UnixMilli()
	conversationDetail := &model.ConversationDetails{
		Id:             uuid.New().String(),
		AssistantId:    req.AssistantId,
		ConversationId: req.ConversationId,
		Prompt:         req.Prompt,
		FileUrl:        req.FileInfo.FileUrl,
		FileSize:       req.FileInfo.FileSize,
		FileName:       req.FileInfo.FileName,
		Response:       response,
		SearchList:     searchList,
		UserId:         req.Identity.UserId,
		OrgId:          req.Identity.OrgId,
		CreatedAt:      nowMilli,
		UpdatedAt:      nowMilli,
	}

	// 写入ES
	if err := es.Assistant().IndexDocument(ctx, indexName, conversationDetail); err != nil {
		return fmt.Errorf("写入ES失败: %v", err)
	}

	log.Infof("成功保存聊天记录到ES，索引: %s, assistantId: %s, conversationId: %s",
		indexName, req.AssistantId, req.ConversationId)
	return nil
}

// ConversationDeleteByAssistantId 根据智能体ID删除对话
func (s *Service) ConversationDeleteByAssistantId(ctx context.Context, req *assistant_service.ConversationDeleteByAssistantIdReq) (*emptypb.Empty, error) {
	if status := s.cli.DeleteConversationByAssistantID(ctx, req.AssistantId, req.Identity.UserId, req.Identity.OrgId); status != nil {
		return nil, errStatus(errs.Code_AssistantConversationErr, status)
	}
	return &emptypb.Empty{}, nil
}
