package knowledge_log

import (
	"context"
	"fmt"
	"time"

	"github.com/UnicomAI/wanwu/internal/knowledge-service/pkg"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/pkg/config"
	"github.com/UnicomAI/wanwu/pkg/log"
	"go.uber.org/zap"
)

// 打印等级 Panic > Error > Warn > Info > Debug

var accessSLog *zap.SugaredLogger

var accessLog = AccessLog{}

type AccessLog struct {
}

func init() {
	pkg.AddContainer(accessLog)
}

func (c AccessLog) LoadType() string {
	return "access-log-config"
}

func (c AccessLog) Load() error {
	configInfo := config.GetConfig()
	logConfig := configInfo.AccessLog
	logger, err := log.InitLogCore(logConfig.Std, logConfig.Level, logConfig.Logs...)
	if err != nil {
		return err
	}
	accessSLog = logger
	return nil
}

func (c AccessLog) StopPriority() int {
	return pkg.DefaultPriority
}

func (c AccessLog) Stop() error {
	return nil
}

func LogAccessPB(ctx context.Context, business string, method string, params interface{}, result interface{}, err error, starTimestamp int64) {
	defer func() {
		if err1 := recover(); err1 != nil {
			fmt.Println(err1)
		}
	}()
	var success = 1
	if err != nil {
		success = 0
	}

	accessSLog.Infof("%s|%s|%d|%d|%+v|%+v", business, method, success, time.Now().UnixMilli()-starTimestamp, params, result)
}
