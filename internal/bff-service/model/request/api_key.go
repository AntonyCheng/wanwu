package request

type GenApiKeyRequest struct {
	AppId   string `json:"appId" validate:"required"`   // 应用id
	AppType string `json:"appType" validate:"required"` // 应用类型
}

func (g GenApiKeyRequest) Check() error {
	return nil
}

type DelApiKeyRequest struct {
	ApiId string `json:"apiId" validate:"required"` // ApiID
}

func (d DelApiKeyRequest) Check() error {
	return nil
}

type GetApiKeyListRequest struct {
	AppId   string `form:"appId" json:"appId" validate:"required"`     // 应用id
	AppType string `form:"appType" json:"appType" validate:"required"` // 应用类型
}

func (g GetApiKeyListRequest) Check() error {
	return nil
}
