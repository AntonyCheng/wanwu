package service

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	err_code "github.com/UnicomAI/wanwu/api/proto/err-code"
	rag_service "github.com/UnicomAI/wanwu/api/proto/rag-service"
	"github.com/UnicomAI/wanwu/internal/bff-service/model/request"
	"github.com/UnicomAI/wanwu/internal/bff-service/pkg/ahocorasick"
	"github.com/UnicomAI/wanwu/pkg/constant"
	grpc_util "github.com/UnicomAI/wanwu/pkg/grpc-util"
	"github.com/UnicomAI/wanwu/pkg/log"
	sse_util "github.com/UnicomAI/wanwu/pkg/sse-util"
	"github.com/UnicomAI/wanwu/pkg/util"
	"github.com/gin-gonic/gin"
)

// ChatRagStream rag私域问答
func ChatRagStream(ctx *gin.Context, userId, orgId string, req request.ChatRagRequest) error {
	// 1.CallRagChatStream
	chatCh, err := CallRagChatStream(ctx, userId, orgId, req)
	if err != nil {
		return err
	}
	// 2.流式返回结果
	_ = sse_util.NewSSEWriter(ctx, fmt.Sprintf("[RAG] %v user %v org %v", req.RagID, userId, orgId), sse_util.DONE_MSG).
		WriteStream(chatCh, nil, buildRagChatRespLineProcessor(), nil)
	return nil
}

// CallRagChatStream 调用Rag对话
func CallRagChatStream(ctx *gin.Context, userId, orgId string, req request.ChatRagRequest) (<-chan string, error) {
	// 根据ragID获取敏感词配置
	ragInfo, err := rag.GetRagDetail(ctx, &rag_service.RagDetailReq{
		RagId: req.RagID,
	})
	if err != nil {
		return nil, err
	}
	var matchDicts []ahocorasick.DictConfig
	// 如果Enable为true,则处理敏感词
	if ragInfo.SensitiveConfig.GetEnable() {
		matchDicts, err = BuildSensitiveDict(ctx, ragInfo.SensitiveConfig.GetTableIds())
		if err != nil {
			return nil, err
		}
		matchResults, err := ahocorasick.ContentMatch(req.Question, matchDicts, true)
		if err != nil {
			return nil, grpc_util.ErrorStatus(err_code.Code_BFFSensitiveWordCheck, err.Error())
		}
		if len(matchResults) > 0 {
			if matchResults[0].Reply != "" {
				return nil, grpc_util.ErrorStatusWithKey(err_code.Code_BFFSensitiveWordCheck, "bff_sensitive_check_req", matchResults[0].Reply)
			}
			return nil, grpc_util.ErrorStatusWithKey(err_code.Code_BFFSensitiveWordCheck, "bff_sensitive_check_req_default_reply")
		}
	}
	stream, err := rag.ChatRag(ctx, &rag_service.ChatRagReq{
		RagId:    req.RagID,
		Question: req.Question,
		Identity: &rag_service.Identity{
			UserId: userId,
			OrgId:  orgId,
		},
	})
	if err != nil {
		return nil, err
	}

	firstResp, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			// 流已经结束，没有数据
			return nil, err
		}
		return nil, err
	}

	rawCh := make(chan string, 128)
	go func() {
		defer util.PrintPanicStack()
		defer close(rawCh)
		log.Infof("[RAG] %v user %v org %v start, query: %s", req.RagID, userId, orgId, req.Question)
		rawCh <- firstResp.Content
		for {
			s, err := stream.Recv()
			if err == io.EOF {
				log.Infof("[RAG] %v user %v org %v stop", req.RagID, userId, orgId)
				break
			}
			if err != nil {
				log.Errorf("[RAG] %v user %v org %v recv err: %v", req.RagID, userId, orgId, err)
				break
			}
			rawCh <- s.Content
		}
	}()
	if !ragInfo.SensitiveConfig.GetEnable() {
		return rawCh, nil
	}
	// 敏感词过滤
	retCh := ProcessSensitiveWords(ctx, rawCh, matchDicts, &ragSensitiveService{})
	return retCh, nil
}

// buildRagChatRespLineProcessor 构造rag对话结果行处理器
func buildRagChatRespLineProcessor() func(*gin.Context, string, interface{}) (string, bool, error) {
	return func(c *gin.Context, lineText string, params interface{}) (string, bool, error) {
		if strings.HasPrefix(lineText, "error:") {
			errorText := fmt.Sprintf("data: {\"code\": -1, \"message\": \"%s\"}\n\n", strings.TrimPrefix(lineText, "error:"))
			return errorText, false, nil
		}
		if strings.HasPrefix(lineText, "data:") {
			return lineText + "\n\n", false, nil
		}
		return lineText + "\n\n", false, nil
	}
}

// --- rag sensitive ---

type ragSensitiveService struct{}

func (s *ragSensitiveService) serviceType() string {
	return constant.AppTypeRag
}

func (s *ragSensitiveService) parseContent(raw string) (id, content string) {
	// 1. 清理数据前缀
	raw = strings.TrimPrefix(raw, "data:")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	// 2. 解析JSON
	resp := struct {
		MsgID string `json:"msg_id"`
		Data  struct {
			Output string `json:"output"`
		} `json:"data"`
	}{}

	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", ""
	}
	// 3. 返回content
	return resp.MsgID, resp.Data.Output
}

func (s *ragSensitiveService) buildSensitiveResp(id string, content string) []string {
	resp := map[string]interface{}{
		"code":    0,
		"message": "success",
		"msg_id":  id,
		"data": map[string]interface{}{
			"output":     content,
			"searchList": []interface{}{},
		},
		"history": []interface{}{},
		"finish":  1,
	}
	marshal, _ := json.Marshal(resp)
	return []string{"data: " + string(marshal)}
}
