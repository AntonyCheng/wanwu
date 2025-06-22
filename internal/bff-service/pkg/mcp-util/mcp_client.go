package mcp_util

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
	"github.com/UnicomAI/wanwu/pkg/log"
)

func ListTools(ctx context.Context, sseUrl string) ([]*protocol.Tool, error) {
	// 创建 SSE 传输客户端
	transportClient, err := transport.NewSSEClientTransport(sseUrl,
		transport.WithSSEClientOptionLogger(log.Log()),
		transport.WithSSEClientOptionHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // 跳过证书验证
				},
			},
		}))
	if err != nil {
		return nil, fmt.Errorf("mcp list tools (%v) init transport err: %v", sseUrl, err)
	}

	// 初始化 MCP 客户端
	mcpClient, err := client.NewClient(transportClient)
	if err != nil {
		return nil, fmt.Errorf("mcp list tools (%v) init client err: %v", sseUrl, err)
	}
	defer mcpClient.Close()

	// 获取可用工具列表
	resp, err := mcpClient.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("mcp list tools (%v) err: %v", sseUrl, err)
	}
	return resp.Tools, nil
}
