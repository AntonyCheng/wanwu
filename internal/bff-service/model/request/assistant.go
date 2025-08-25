package request

import "fmt"

type AssistantBrief struct {
	AssistantId string `json:"assistantId"  validate:"required"`
	AppBriefConfig
}

func (a *AssistantBrief) Check() error { return nil }

type AssistantConfig struct {
	AssistantId         string                 `json:"assistantId"  validate:"required"`
	Prologue            string                 `json:"prologue"`            // 开场白
	Instructions        string                 `json:"instructions"`        // 系统提示词
	RecommendQuestion   []string               `json:"recommendQuestion"`   // 推荐问题
	ModelConfig         AppModelConfig         `json:"modelConfig"`         // 模型
	KnowledgeBaseConfig AppKnowledgebaseConfig `json:"knowledgeBaseConfig"` // 知识库
	SafetyConfig        AppSafetyConfig        `json:"safetyConfig"`        // 敏感词表配置
	RerankConfig        AppModelConfig         `json:"rerankConfig"`        // Rerank模型
	OnlineSearchConfig  OnlineSearchConfig     `json:"onlineSearchConfig"`  // 在线搜索
}

func (a *AssistantConfig) Check() error { return nil }

type AssistantPublish struct {
	AssistantId string `json:"assistantId"  validate:"required"`
	Scope       int32  `json:"scope"  validate:"required"`
}

func (a *AssistantPublish) Check() error { return nil }

type AssistantDeleteRequest struct {
	AssistantId string `json:"assistantId"  validate:"required"`
}

func (a *AssistantDeleteRequest) Check() error { return nil }

type AssistantIdRequest struct {
	AssistantId string `json:"assistantId" form:"assistantId" validate:"required"`
}

func (a *AssistantIdRequest) Check() error { return nil }

type AssistantWorkFlowAddRequest struct {
	AssistantId string `json:"assistantId" validate:"required"`
	WorkFlowId  string `json:"workFlowId" validate:"required"`
}

func (w *AssistantWorkFlowAddRequest) Check() error { return nil }

type AssistantWorkFlowDelRequest struct {
	AssistantId string `json:"assistantId" validate:"required"`
	WorkFlowId  string `json:"workFlowId" validate:"required"`
}

func (w *AssistantWorkFlowDelRequest) Check() error { return nil }

type AssistantWorkFlowToolEnableRequest struct {
	AssistantId string `json:"assistantId" validate:"required"`
	WorkFlowId  string `json:"workFlowId" validate:"required"`
	Enable      bool   `json:"enable"`
}

func (w *AssistantWorkFlowToolEnableRequest) Check() error { return nil }

type AssistantMCPToolAddRequest struct {
	AssistantId string `json:"assistantId" validate:"required"`
	MCPId       string `json:"mcpId" validate:"required"`
}

func (m *AssistantMCPToolAddRequest) Check() error { return nil }

type AssistantMCPToolDelRequest struct {
	AssistantId string `json:"assistantId" validate:"required"`
	MCPId       string `json:"mcpId" validate:"required"`
}

func (w *AssistantMCPToolDelRequest) Check() error { return nil }

type AssistantMCPToolEnableRequest struct {
	AssistantId string `json:"assistantId" validate:"required"`
	MCPId       string `json:"mcpId" validate:"required"`
	Enable      bool   `json:"enable"`
}

func (a *AssistantMCPToolEnableRequest) Check() error { return nil }

type ActionAddRequest struct {
	AssistantId string            `json:"assistantId"  validate:"required"`
	Schema      string            `json:"schema"  validate:"required"`
	ApiAuth     ApiAuthWebRequest `json:"apiAuth" validate:"required"`
}

func (a *ActionAddRequest) Check() error { return nil }

type ApiAuthWebRequest struct {
	Type             string `json:"type"`
	APIKey           string `json:"apiKey"`
	CustomHeaderName string `json:"customHeaderName"`
	AuthType         string `json:"authType"`
}

type ActionUpdateRequest struct {
	ActionId string            `json:"actionId"  validate:"required"`
	Schema   string            `json:"schema"  validate:"required"`
	ApiAuth  ApiAuthWebRequest `json:"apiAuth"  validate:"required"`
}

func (a *ActionUpdateRequest) Check() error { return nil }

type ActionIdRequest struct {
	ActionId string `json:"actionId" form:"actionId"  validate:"required"`
}

func (a *ActionIdRequest) Check() error { return nil }

type ConversationCreateRequest struct {
	AssistantId string `json:"assistantId"  validate:"required"`
	Prompt      string `json:"prompt"  validate:"required"`
}

func (c *ConversationCreateRequest) Check() error { return nil }

type ConversationIdRequest struct {
	ConversationId string `json:"conversationId" form:"conversationId"  validate:"required"`
}

func (c *ConversationIdRequest) Check() error { return nil }

type ConversationGetListRequest struct {
	AssistantId string `json:"assistantId" form:"assistantId"  validate:"required"`
	PageSize    int    `json:"pageSize" form:"pageSize"  validate:"required"`
	PageNo      int    `json:"pageNo" form:"pageNo"  validate:"required"`
}

func (c *ConversationGetListRequest) Check() error { return nil }

type ConversationGetDetailListRequest struct {
	ConversationId string `json:"conversationId" form:"conversationId"  validate:"required"`
	PageSize       int    `json:"pageSize" form:"pageSize"  validate:"required"`
	PageNo         int    `json:"pageNo" form:"pageNo"  validate:"required"`
}

func (c *ConversationGetDetailListRequest) Check() error { return nil }

type ConversionStreamRequest struct {
	AssistantId    string               `json:"assistantId" form:"assistantId"  validate:"required"`
	ConversationId string               `json:"conversationId" form:"conversionId"`
	FileInfo       ConversionStreamFile `json:"fileInfo" form:"fileInfo"`
	Trial          bool                 `json:"trial" form:"trial"`
	Prompt         string               `json:"prompt" form:"prompt"  validate:"required"`
}

func (c *ConversionStreamRequest) Check() error {
	// 当Trial=false时，ConversationId必填
	if !c.Trial && c.ConversationId == "" {
		return fmt.Errorf("conversationId is required when trial is false")
	}
	return nil
}

type ConversionStreamFile struct {
	FileName string `json:"fileName" form:"fileName"`
	FileSize int64  `json:"fileSize" form:"fileSize"`
	FileUrl  string `json:"fileUrl" form:"fileUrl"`
}

type OnlineSearchConfig struct {
	SearchUrl      string `json:"searchUrl" form:"searchUrl"`
	SearchKey      string `json:"searchKey" form:"searchKey"`
	SearchRerankId string `json:"searchRerankId" form:"searchRerankId"`
	Enable         bool   `json:"enable" form:"enable"`
}

func (o *OnlineSearchConfig) Check() error {
	if (o.SearchUrl == "" && o.SearchKey != "") || (o.SearchUrl != "" && o.SearchKey == "") {
		return fmt.Errorf("searchUrl and searchKey must be set together")
	}
	return nil
}

type AssistantTemplateRequest struct {
	AssistantTemplateId string `json:"assistantTemplateId" form:"assistantTemplateId"  validate:"required"`
}

func (a *AssistantTemplateRequest) Check() error { return nil }

type AssistantCustomToolAddRequest struct {
	AssistantId  string `json:"assistantId" validate:"required"`
	CustomToolId string `json:"customToolId" validate:"required"`
}

func (c *AssistantCustomToolAddRequest) Check() error { return nil }

type AssistantCustomToolDelRequest struct {
	AssistantId  string `json:"assistantId" validate:"required"`
	CustomToolId string `json:"customToolId" validate:"required"`
}

func (c *AssistantCustomToolDelRequest) Check() error { return nil }

type AssistantCustomToolEnableRequest struct {
	AssistantId  string `json:"assistantId" validate:"required"`
	CustomToolId string `json:"customToolId" validate:"required"`
	Enable       bool   `json:"enable"`
}

func (c *AssistantCustomToolEnableRequest) Check() error { return nil }
