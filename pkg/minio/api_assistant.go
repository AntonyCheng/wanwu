package minio

import (
	"context"
	"fmt"
)

var (
	_minioAssistant *client
)

func InitAssistant(ctx context.Context, cfg Config, initBucketName string) error {
	if _minioAssistant != nil {
		return fmt.Errorf("minio assistat client already init")
	}
	c, err := newClient(cfg)
	if err != nil {
		return err
	}
	_minioAssistant = c

	// 创建存储桶并设置存储策略
	if _, err = _minioAssistant.createBucketIfAbsent(ctx, initBucketName); err != nil {
		return err
	}

	return nil
}
