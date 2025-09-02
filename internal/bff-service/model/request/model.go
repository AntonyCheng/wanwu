package request

type ImportOrUpdateModelRequest struct {
	ModelConfig
}

func (o *ImportOrUpdateModelRequest) Check() error {
	return o.ModelConfig.Check()
}

type DeleteModelRequest struct {
	BaseModelRequest
}

func (o *DeleteModelRequest) Check() error {
	return nil
}

type GetModelRequest struct {
	BaseModelRequest
}

func (o *GetModelRequest) Check() error {
	return nil
}

type ListModelsRequest struct {
	ModelType   string `json:"modelType" form:"modelType" `    // 模型类型
	Provider    string `json:"provider" form:"provider"`       // 模型供应商
	DisplayName string `json:"displayName" form:"displayName"` // 模型显示名称
	IsActive    bool   `json:"isActive" form:"isActive"`       // 启用状态（true: 启用）
}

func (o *ListModelsRequest) Check() error {
	return nil
}

type ListTypeModelsRequest struct {
	ModelType string `json:"modelType" form:"modelType" ` // 模型类型
}

func (o *ListTypeModelsRequest) Check() error {
	return nil
}

type ModelStatusRequest struct {
	BaseModelRequest
	IsActive bool `json:"isActive"` // 启用状态（true: 启用，false: 禁用）
}

func (o *ModelStatusRequest) Check() error {
	return nil
}

type GetModelByIdRequest struct {
	BaseModelRequest
}

func (o *GetModelByIdRequest) Check() error {
	return nil
}
