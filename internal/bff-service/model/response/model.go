package response

import (
	"github.com/UnicomAI/wanwu/internal/bff-service/model/request"
	mp "github.com/UnicomAI/wanwu/pkg/model-provider"
)

type ModelInfo struct {
	ModelId     string         `json:"modelId"`
	Provider    string         `json:"provider" validate:"required" enums:"OpenAI-API-compatible,YuanJing"` // 模型供应商
	Model       string         `json:"model" validate:"required"`                                           // 模型名称
	ModelType   string         `json:"modelType" validate:"required" enums:"llm,embedding,rerank"`
	DisplayName string         `json:"displayName"` // 模型显示名称
	Avatar      request.Avatar `json:"avatar" `     // 模型图标路径
	PublishDate string         `json:"publishDate"` // 模型发布时间
	IsActive    bool           `json:"isActive"`    // 启用状态（true: 启用，false: 禁用）
	UserId      string         `json:"userId"`
	OrgId       string         `json:"orgId"`
	CreatedAt   string         `json:"createdAt"`
	UpdatedAt   string         `json:"updatedAt"`
	Config      interface{}    `json:"config"`

	Examples *mp.ProviderModelConfig `json:"examples,omitempty"` // 仅用于swagger展示；模型对应供应商中的对应llm、embedding或rerank结构是config实际的参数
}
