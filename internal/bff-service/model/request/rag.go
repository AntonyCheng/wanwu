package request

type RagBrief struct {
	RagID string `json:"ragId" validate:"required"`
	AppBriefConfig
}

type RagConfig struct {
	RagID               string                 `json:"ragId" validate:"required"`
	ModelConfig         AppModelConfig         `json:"modelConfig" validate:"required"`         // 模型
	RerankConfig        AppModelConfig         `json:"rerankConfig" validate:"required"`        // Rerank模型
	KnowledgeBaseConfig AppKnowledgebaseConfig `json:"knowledgeBaseConfig" validate:"required"` // 知识库
}

type ChatRagRequest struct {
	RagID    string `json:"ragId" validate:"required"`
	Question string `json:"question" validate:"required"`
}

type RagReq struct {
	RagID string `form:"ragId" json:"ragId" validate:"required"`
}

func (r RagBrief) Check() error {
	return nil
}

func (r RagConfig) Check() error {
	if err := r.ModelConfig.Check(); err != nil {
		return err
	}
	if err := r.RerankConfig.Check(); err != nil {
		return err
	}
	return nil
}

func (c ChatRagRequest) Check() error {
	return nil
}

func (r RagReq) Check() error {
	return nil
}
