package minio

import (
	"context"

	"github.com/UnicomAI/wanwu/internal/knowledge-service/pkg"
	"github.com/UnicomAI/wanwu/internal/knowledge-service/pkg/config"
	"github.com/UnicomAI/wanwu/pkg/minio"
)

var minioClient = ClientMinio{}

type ClientMinio struct {
}

func init() {
	pkg.AddContainer(minioClient)
}

func (c ClientMinio) LoadType() string {
	return "minioClient"
}

func (c ClientMinio) Load() error {
	minioConfig := config.GetConfig().Minio
	return minio.InitKnowledge(context.Background(), minio.Config{
		Endpoint: minioConfig.EndPoint,
		User:     minioConfig.User,
		Password: minioConfig.Password,
	}, minioConfig.Bucket)
}

func (c ClientMinio) StopPriority() int {
	return pkg.DefaultPriority
}

func (c ClientMinio) Stop() error {
	return nil
}
