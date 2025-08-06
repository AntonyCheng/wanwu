package client

import (
	"context"

	err_code "github.com/UnicomAI/wanwu/api/proto/err-code"
	"github.com/UnicomAI/wanwu/internal/operate-service/client/orm"
)

type IClient interface {
	// 系统自定义配置
	CreateSystemCustom(ctx context.Context, userID, orgID string, key orm.SystemCustomKey, mode orm.SystemCustomMode, custom orm.SystemCustom) *err_code.Status
	GetSystemCustom(ctx context.Context, mode orm.SystemCustomMode) (*orm.SystemCustom, *err_code.Status)
}
