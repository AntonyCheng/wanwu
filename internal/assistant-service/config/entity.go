package config

type AgentScopeWorkFlowSchemaResp struct {
	Code int `json:"code"`
	Data struct {
		Base64OpenAPISchema string `json:"base64OpenAPISchema"`
	} `json:"data"`
}

type AssistantConversionHistory struct {
	Query         string `json:"query"`
	UploadFileUrl string `json:"upload_file_url,omitempty"`
	Response      string `json:"response"`
}

type KnParams struct {
	KnowledgeBase  []string    `json:"knowledgeBase"`
	RerankId       interface{} `json:"rerank_id"`
	Model          interface{} `json:"model"`
	ModelUrl       interface{} `json:"model_url"`
	RerankMod      string      `json:"rerank_mod"`
	RetrieveMethod string      `json:"retrieve_method"`
	Weights        []float64   `json:"weights,omitempty"`
	MaxHistory     int         `json:"max_history"`
	Threshold      float32     `json:"threshold"`
	TopK           int         `json:"topK"`
	RewriteQuery   bool        `json:"rewrite_query"`
	TermWeight     float32     `json:"term_weight_coefficient"` // 关键词系数, 默认为1
}

type AgentSSERequest struct {
	Input          string                       `json:"input"`
	Stream         bool                         `json:"stream"`
	SystemRole     string                       `json:"system_role,omitempty"`
	UploadFileUrl  string                       `json:"upload_file_url,omitempty"`
	FileName       string                       `json:"file_name,omitempty"`
	PluginList     []PluginListAlgRequest       `json:"plugin_list,omitempty"`
	Model          string                       `json:"model,omitempty"`
	ModelUrl       string                       `json:"model_url,omitempty"`
	SearchUrl      string                       `json:"search_url,omitempty"`
	SearchKey      string                       `json:"search_key,omitempty"`
	SearchRerankId interface{}                  `json:"search_rerank_id,omitempty"`
	UseSearch      bool                         `json:"use_search,omitempty"`
	KnParams       *KnParams                    `json:"kn_params,omitempty"`
	UseKnow        bool                         `json:"use_know,omitempty"`
	ModelId        string                       `json:"model_id,omitempty"`
	History        []AssistantConversionHistory `json:"history,omitempty"`
	McpTools       map[string]MCPToolInfo       `json:"mcp_tools,omitempty"`
	AutoCitation   bool                         `json:"auto_citation,omitempty"`
	ModelParams    map[string]interface{}       `json:"-"` // 用于合并动态模型参数，不直接序列化到JSON
}

type PluginListAlgRequest struct {
	APISchema map[string]interface{} `json:"api_schema"`
	APIAuth   *APIAuth               `json:"api_auth,omitempty"`
}

type APIAuth struct {
	Type  string `json:"type"`
	In    string `json:"in"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type MCPToolInfo struct {
	URL       string `json:"url"`
	Transport string `json:"transport"`
}

type ToolsMap map[string]MCPToolInfo

type RequestData struct {
	McpTools ToolsMap `json:"mcp_tools"`
}
