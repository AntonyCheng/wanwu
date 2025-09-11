package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"

	model_service "github.com/UnicomAI/wanwu/api/proto/model-service"
	"github.com/UnicomAI/wanwu/internal/bff-service/config"
	"github.com/UnicomAI/wanwu/pkg/log"
	mp "github.com/UnicomAI/wanwu/pkg/model-provider"
	mp_common "github.com/UnicomAI/wanwu/pkg/model-provider/mp-common"
	"github.com/gin-gonic/gin"
)

// 定义校验函数类型
type ModelValidator func(ctx *gin.Context, modelInfo *model_service.ModelInfo) error

// 校验器注册表
var validators = sync.OnceValue(func() map[string]ModelValidator {
	return map[string]ModelValidator{
		mp.ModelTypeLLM:       ValidateLLMModel,
		mp.ModelTypeRerank:    ValidateRerankModel,
		mp.ModelTypeEmbedding: ValidateEmbeddingModel,
		mp.ModelTypeOcr:       ValidateOcrModel,
		mp.ModelTypeGui:       ValidateGuideModel,
	}
})

// 统一校验入口
func ValidateModel(ctx *gin.Context, modelInfo *model_service.ModelInfo) error {
	validator, exists := validators()[strings.ToLower(modelInfo.ModelType)]
	if !exists {
		return fmt.Errorf("unsupported model type: %s", modelInfo.ModelType)
	}
	return validator(ctx, modelInfo)
}

func ValidateLLMModel(ctx *gin.Context, modelInfo *model_service.ModelInfo) error {
	llm, err := mp.ToModelConfig(modelInfo.Provider, modelInfo.ModelType, modelInfo.ProviderConfig)
	if err != nil {
		return err
	}
	iLLM, ok := llm.(mp.ILLM)
	if !ok {
		return fmt.Errorf("invalid provider")
	}
	// mock  request
	var stream bool = false
	req := &mp_common.LLMReq{
		Model: modelInfo.Model,
		Messages: []mp_common.OpenAIMsg{
			{
				Role:    mp_common.MsgRoleUser,
				Content: "几点了",
			},
		},
		Stream: &stream,
	}
	// ToolCall 校验
	var result map[string]interface{}
	err = json.Unmarshal([]byte(modelInfo.ProviderConfig), &result)
	if err != nil {
		return err
	}
	fc, ok := result["functionCalling"].(string)
	if ok && mp_common.FCType(fc) == mp_common.FCTypeToolCall {
		tools := []mp_common.OpenAITool{
			{
				Type: mp_common.ToolTypeFunction,
				Function: &mp_common.OpenAIFunction{
					Name:        "get_current_time",
					Description: "当你想知道现在的时间时非常有用。",
					Parameters: &mp_common.OpenAIFunctionParameters{
						Type:       "object",
						Properties: map[string]mp_common.OpenAIFunctionParametersProperty{},
					},
				},
			},
		}
		req.Tools = tools
		llmReq, err := iLLM.NewReq(req)
		if err != nil {
			return err
		}
		resp, _, err := iLLM.ChatCompletions(ctx.Request.Context(), llmReq)
		if err != nil {
			return err
		}
		openAIResp, ok := resp.ConvertResp()
		if !ok {
			return fmt.Errorf("invalid resp: %v", err)
		}
		if len(openAIResp.Choices) == 0 || openAIResp.Choices[0].Message.ToolCalls == nil {
			return fmt.Errorf("model does not support toolcall functionality")
		} else {
			data, _ := json.MarshalIndent(openAIResp.Choices[0].Message.ToolCalls, "", "  ")
			log.Debugf("tool call: %v", string(data))
		}
		return nil
	}
	llmReq, err := iLLM.NewReq(req)
	if err != nil {
		return err
	}
	resp, _, err := iLLM.ChatCompletions(ctx.Request.Context(), llmReq)
	if err != nil {
		return fmt.Errorf("model API call failed: %v", err)
	}
	_, ok = resp.ConvertResp()
	if !ok {
		return fmt.Errorf("invalid response format")
	}
	return nil
}

func ValidateEmbeddingModel(ctx *gin.Context, modelInfo *model_service.ModelInfo) error {
	embedding, err := mp.ToModelConfig(modelInfo.Provider, modelInfo.ModelType, modelInfo.ProviderConfig)
	if err != nil {
		return err
	}
	iEmbedding, ok := embedding.(mp.IEmbedding)
	if !ok {
		return fmt.Errorf("invalid provider")
	}
	// mock  request
	req := &mp_common.EmbeddingReq{
		Model: modelInfo.Model,
		Input: []string{"你好"},
	}
	embeddingReq, err := iEmbedding.NewReq(req)
	if err != nil {
		return err
	}
	resp, err := iEmbedding.Embeddings(ctx.Request.Context(), embeddingReq)
	if err != nil {
		{
			return fmt.Errorf("model API call failed: %v", err)
		}
	}
	_, ok = resp.ConvertResp()
	if !ok {
		return fmt.Errorf("invalid response format")
	}
	return nil
}

func ValidateRerankModel(ctx *gin.Context, modelInfo *model_service.ModelInfo) error {
	rerank, err := mp.ToModelConfig(modelInfo.Provider, modelInfo.ModelType, modelInfo.ProviderConfig)
	if err != nil {
		return err
	}
	iRerank, ok := rerank.(mp.IRerank)
	if !ok {
		return fmt.Errorf("invalid provider")
	}
	// mock  request
	req := &mp_common.RerankReq{
		Model: modelInfo.Model,
		Query: "乌萨奇",
		Documents: []string{
			"乌萨奇",
			"尖尖我噶奶～",
		},
	}
	rerankReq, err := iRerank.NewReq(req)
	if err != nil {
		return err
	}
	resp, err := iRerank.Rerank(ctx.Request.Context(), rerankReq)
	if err != nil {
		return fmt.Errorf("model API call failed: %v", err)
	}
	_, ok = resp.ConvertResp()
	if !ok {
		return fmt.Errorf("invalid response format")
	}
	return nil
}

func ValidateOcrModel(ctx *gin.Context, modelInfo *model_service.ModelInfo) error {
	rerank, err := mp.ToModelConfig(modelInfo.Provider, modelInfo.ModelType, modelInfo.ProviderConfig)
	if err != nil {
		return err
	}
	iOcr, ok := rerank.(mp.IOcr)
	if !ok {
		return fmt.Errorf("invalid provider")
	}
	// mock  request

	file, err := os.Open(config.Cfg().Model.OcrFilePath)
	if err != nil {
		return fmt.Errorf("open file failed: %v", err)
	}
	defer file.Close()

	// 创建内存缓冲区
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 创建表单文件字段
	part, err := writer.CreateFormFile("file", file.Name())
	if err != nil {
		return fmt.Errorf("create form file failed: %v", err)
	}

	// 复制文件内容
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copy file content failed: %v", err)
	}
	writer.Close()

	// 模拟HTTP请求
	mockReq, _ := http.NewRequest("POST", "", body)
	mockReq.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = mockReq
	// 获取FileHeader对象
	_, fileH, err := ctx.Request.FormFile("file")
	if err != nil {
		return fmt.Errorf("get file header failed: %v", err)
	}
	req := &mp_common.OcrReq{
		Files: fileH,
	}
	ocrReq, err := iOcr.NewReq(req)
	if err != nil {
		return err
	}
	resp, err := iOcr.Ocr(ctx, ocrReq)
	if err != nil {
		return fmt.Errorf("model API call failed: %v", err)
	}
	_, ok = resp.ConvertResp()
	if !ok {
		return fmt.Errorf("invalid response format")
	}
	return nil
}

func ValidateGuideModel(ctx *gin.Context, modelInfo *model_service.ModelInfo) error {
	gui, err := mp.ToModelConfig(modelInfo.Provider, modelInfo.ModelType, modelInfo.ProviderConfig)
	if err != nil {
		return err
	}
	iGui, ok := gui.(mp.IGui)
	if !ok {
		return fmt.Errorf("invalid provider")
	}
	// mock  request
	// 读取图片文件
	imageFile := config.Cfg().Model.OcrFilePath
	imageBytes, err := os.ReadFile(imageFile)
	if err != nil {
		return fmt.Errorf("ReadFile file failed: %v", err)
	}

	// 转换为base64字符串
	imageBase64 := base64.StdEncoding.EncodeToString(imageBytes)
	height, width := 931, 144
	req := &mp_common.GuiReq{
		Algo:                    "gui_agent_v1",
		Platform:                "Mobile",
		CurrentScreenshot:       "data:image/jpeg;base64," + imageBase64,
		CurrentScreenshotHeight: height,
		CurrentScreenshotWidth:  width,
		Task:                    "点击屏幕以开始",
		History:                 []string{},
	}
	guiReq, err := iGui.NewReq(req)
	if err != nil {
		return err
	}
	resp, err := iGui.Gui(ctx.Request.Context(), guiReq)
	if err != nil {
		return fmt.Errorf("model API call failed: %v", err)
	}
	_, ok = resp.ConvertResp()
	if !ok {
		return fmt.Errorf("invalid response format")
	}
	return nil
}
