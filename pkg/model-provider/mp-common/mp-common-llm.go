package mp_common

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/UnicomAI/wanwu/pkg/log"
	"github.com/UnicomAI/wanwu/pkg/util"
	"github.com/go-resty/resty/v2"
)

type MsgRole string

const (
	MsgRoleSystem    MsgRole = "system"
	MsgRoleUser      MsgRole = "user"
	MsgRoleAssistant MsgRole = "assistant"
	MsgRoleFunction  MsgRole = "tool"
)

type ToolType string

const (
	ToolTypeFunction ToolType = "function"
)

type FCType string

const (
	FCTypeFunctionCall FCType = "functionCall"
	FCTypeNoSupport    FCType = "noSupport"
	FCTypeToolCall     FCType = "toolCall"
)

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// --- openapi request ---

type LLMReq struct {
	// general
	Model          string                `json:"model" validate:"required"`
	Messages       []OpenAIMsg           `json:"messages" validate:"required"`
	Stream         *bool                 `json:"stream,omitempty"`
	MaxTokens      *int                  `json:"max_tokens,omitempty"`
	Stop           *string               `json:"stop,omitempty"`
	ResponseFormat *OpenAIResponseFormat `json:"response_format,omitempty"`
	Temperature    *float64              `json:"temperature,omitempty"`
	Tools          []OpenAITool          `json:"tools,omitempty"`

	// custom
	Thinking            *Thinking      `json:"thinking,omitempty"` // 控制模型是否开启深度思考模式。
	EnableThinking      *bool          `json:"enable_thinking,omitempty"`
	MaxCompletionTokens *int           `json:"max_completion_tokens,omitempty"` // 控制模型输出的最大长度[0,64k]
	LogitBias           map[string]int `json:"logit_bias,omitempty"`            // 调整指定 token 在模型输出内容中出现的概率
	ToolChoice          interface{}    `json:"tool_choice,omitempty"`           // 强制指定工具调用的策略
	TopP                *float64       `json:"top_p,omitempty"`
	TopK                *int           `json:"top_k,omitempty"`
	MinP                *float64       `json:"min_p,omitempty"`
	ParallelToolCalls   *bool          `json:"parallel_tool_calls,omitempty"` // 是否开启并行工具调用
	StreamOptions       *StreamOptions `json:"stream_options,omitempty"`      //当启用流式输出时，可通过将本参数设置为{"include_usage": true}，在输出的最后一行显示所使用的Token数。

	PresencePenalty   *float64 `json:"presence_penalty,omitempty"`  // 控制模型生成文本时的内容重复度
	FrequencyPenalty  *float64 `json:"frequency_penalty,omitempty"` // 频率惩罚系数
	RepetitionPenalty *float64 `json:"repetition_penalty,omitempty"`

	Seed           *int  `json:"seed,omitempty"`
	Logprobs       *bool `json:"logprobs,omitempty"`     // 是否返回输出 Token 的对数概率
	TopLogprobs    *int  `json:"top_logprobs,omitempty"` // 指定在每一步生成时，返回模型最大概率的候选 Token 个数
	N              *int  `json:"n,omitempty"`
	ThinkingBudget *int  `json:"thinking_budget,omitempty"` // 思考过程的最大长度，只在enable_thinking为true时生效

	WebSearch *WebSearch `json:"web_search,omitempty"` //搜索增强
	User      *string    `json:"user,omitempty"`
	// Yuanjing
	DoSample *bool `json:"do_sample,omitempty"`
}

func (req *LLMReq) Data() (map[string]interface{}, error) {
	m := make(map[string]interface{})
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

type StreamOptions struct {
	IncludeUsage      *bool `json:"include_usage,omitempty"`
	ChunkIncludeUsage *bool `json:"chunk_include_usage,omitempty"`
}

type WebSearch struct {
	Enable         *bool `json:"enable,omitempty"`
	EnableCitation *bool `json:"enable_citation,omitempty"`
	EnableTrace    *bool `json:"enable_trace,omitempty"`
	EnableStatus   *bool `json:"enable_status,omitempty"`
}

type OpenAIMsg struct {
	Role             MsgRole       `json:"role" validate:"required"` // "system" | "user" | "assistant" | "function(已弃用)"
	Content          string        `json:"content" validate:"required"`
	ToolCallId       *string       `json:"tool_call_id,omitempty"`
	ReasoningContent *string       `json:"reasoning_content,omitempty"`
	Name             *string       `json:"name,omitempty"`
	FunctionCall     *FunctionCall `json:"function_call,omitempty"`
	ToolCalls        []*ToolCall   `json:"tool_calls,omitempty"`
}

type Thinking struct {
	Type string `json:"type" default:"enabled"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     ToolType     `json:"type"`
	Function FunctionCall `json:"function"`
	Index    *int         `json:"index,omitempty"`
}

type FunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}
type OpenAIResponseFormat struct {
	Type string `json:"type"` // "text" | "json"
}

type OpenAITool struct {
	Type     ToolType        `json:"type" validate:"required"`
	Function *OpenAIFunction `json:"function" validate:"required"`
}

type OpenAIFunction struct {
	Name        string                    `json:"name" validate:"required"`
	Description string                    `json:"description,omitempty"`
	Parameters  *OpenAIFunctionParameters `json:"parameters,omitempty"`
}

type OpenAIFunctionParameters struct {
	Type       string                                      `json:"type"`
	Properties map[string]OpenAIFunctionParametersProperty `json:"properties"`
	Required   []string                                    `json:"required"`
}
type OpenAIFunctionParametersProperty struct {
	Description string `json:"description"`
	Type        string `json:"type"`
}

func (req *LLMReq) Check() error { return nil }

// --- openapi response ---

type LLMResp struct {
	ID                string             `json:"id"`           // 唯一标识
	Object            string             `json:"object"`       // 固定为 "chat.completion"
	Created           int                `json:"created"`      // 时间戳（秒）
	Model             string             `json:"model"`        // 使用的模型
	Choices           []OpenAIRespChoice `json:"choices"`      // 生成结果列表
	Usage             OpenAIRespUsage    `json:"usage"`        // token 使用统计
	ServiceTier       *string            `json:"service_tier"` // （火山）指定是否使用TPM保障包。生效对象为购买了保障包推理接入点
	SystemFingerprint *string            `json:"system_fingerprint"`
}

// OpenAIRespUsage 结构体表示 token 消耗
type OpenAIRespUsage struct {
	CompletionTokens int `json:"completion_tokens"` // 输出 token 数
	PromptTokens     int `json:"prompt_tokens"`     // 输入 token 数
	TotalTokens      int `json:"total_tokens"`      // 总 token 数
}

// OpenAIRespChoice 结构体表示单个生成选项
type OpenAIRespChoice struct {
	Index        int         `json:"index"`             // 选项索引
	Message      *OpenAIMsg  `json:"message,omitempty"` // 非流式生成的消息
	Delta        *OpenAIMsg  `json:"delta,omitempty"`   // 流式生成的消息
	FinishReason string      `json:"finish_reason"`     // 停止原因
	Logprobs     interface{} `json:"logprobs"`
}

type OpenAIRespChoiceMsg struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

// --- request ---

type ILLMReq interface {
	Stream() bool
	Data() map[string]interface{}
	OpenAIReq() (*LLMReq, bool)
}

// llmReq implementation of ILLMReq
type llmReq struct {
	data map[string]interface{}
}

func NewLLMReq(data map[string]interface{}) ILLMReq {
	return &llmReq{data: data}
}

func (req *llmReq) Data() map[string]interface{} {
	return req.data
}

func (req *llmReq) Stream() bool {
	if req.data == nil {
		return false
	}
	v, ok := req.data["stream"]
	if !ok {
		return false
	}
	stream, _ := v.(bool)
	return stream
}

func (req *llmReq) OpenAIReq() (*LLMReq, bool) {
	if req == nil {
		return nil, false
	}
	b, err := json.Marshal(req.data)
	if err != nil {
		log.Errorf("LLMReq to LLMReq marshal err: %v", err)
		return nil, false
	}
	ret := &LLMReq{}
	if err = json.Unmarshal(b, ret); err != nil {
		log.Errorf("LLMReq to LLMReq unmarshal err: %v", err)
		return nil, false
	}
	return ret, true
}

// --- response ---

type ILLMResp interface {
	String() string
	Data() (map[string]interface{}, bool)
	ConvertResp() (*LLMResp, bool)
}

// llmResp implementation of ILLMResp
type llmResp struct {
	stream bool
	raw    string
}

func NewLLMResp(stream bool, raw string) ILLMResp {
	return &llmResp{stream: stream, raw: raw}
}

func (resp *llmResp) String() string {
	return resp.raw
}

func (resp *llmResp) Data() (map[string]interface{}, bool) {
	if resp.stream {
		if resp.raw == "data: [DONE]" || !strings.HasPrefix(resp.raw, "data:") {
			return nil, false
		}
	}

	raw := resp.raw
	if resp.stream {
		raw = strings.TrimPrefix(resp.raw, "data:")
	}

	ret := make(map[string]interface{})
	if err := json.Unmarshal([]byte(raw), &ret); err != nil {
		log.Errorf("llm stream resp (%v) convert to data err: %v", raw, err)
		return nil, false
	}
	return ret, true
}

func (resp *llmResp) ConvertResp() (*LLMResp, bool) {
	if resp.stream {
		if resp.raw == "data: [DONE]" || !strings.HasPrefix(resp.raw, "data:") {
			return nil, false
		}
	}

	raw := resp.raw
	if resp.stream {
		raw = strings.TrimPrefix(resp.raw, "data:")
	}

	ret := &LLMResp{}
	if err := json.Unmarshal([]byte(raw), ret); err != nil {
		log.Errorf("llm stream resp (%v) convert to openai resp err: %v", raw, err)
		return nil, false
	}
	return ret, true
}

// --- ChatCompletions ---

func ChatCompletions(ctx context.Context, provider, apiKey, url string, req ILLMReq, respConverter func(bool, string) ILLMResp, headers ...Header) (ILLMResp, <-chan ILLMResp, error) {
	if req.Stream() {
		ret, err := chatCompletionsStream(ctx, provider, apiKey, url, req, respConverter, headers...)
		return nil, ret, err
	}
	ret, err := chatCompletionsUnary(ctx, provider, apiKey, url, req, respConverter, headers...)
	return ret, nil, err
}

func chatCompletionsUnary(ctx context.Context, provider, apiKey, url string, req ILLMReq, respConverter func(bool, string) ILLMResp, headers ...Header) (ILLMResp, error) {
	if req.Stream() {
		return nil, fmt.Errorf("request %v %v chat completions unary but stream", url, provider)
	}

	if apiKey != "" {
		headers = append(headers, Header{
			Key:   "Authorization",
			Value: "Bearer " + apiKey,
		})
	}

	request := resty.New().
		SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}). // 关闭证书校验
		SetTimeout(0).                                             // 关闭请求超时
		R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(req.Data()).
		SetDoNotParseResponse(true)
	for _, header := range headers {
		request.SetHeader(header.Key, header.Value)
	}
	resp, err := request.Post(url)
	if err != nil {
		return nil, fmt.Errorf("request %v %v chat completions unary err: %v", url, provider, err)
	} else if resp.StatusCode() >= 300 {
		return nil, fmt.Errorf("request %v %v chat completions unary http status %v msg: %v", url, provider, resp.StatusCode(), resp.String())
	}
	b, err := io.ReadAll(resp.RawResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("request %v %v chat completions unary read response body err: %v", url, provider, err)
	}
	return respConverter(false, string(b)), nil
}

func chatCompletionsStream(ctx context.Context, provider, apiKey, url string, req ILLMReq, respConverter func(bool, string) ILLMResp, headers ...Header) (<-chan ILLMResp, error) {
	if !req.Stream() {
		return nil, fmt.Errorf("request %v %v chat completions stream but unary", url, provider)
	}

	if apiKey != "" {
		headers = append(headers, Header{
			Key:   "Authorization",
			Value: "Bearer " + apiKey,
		})
	}

	ret := make(chan ILLMResp, 1024)
	go func() {
		defer util.PrintPanicStack()
		defer close(ret)
		request := resty.New().
			SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}). // 关闭证书校验
			R().
			SetContext(ctx).
			SetHeader("Content-Type", "application/json").
			SetHeader("Accept", "application/json").
			SetBody(req.Data()).
			SetDoNotParseResponse(true)
		for _, header := range headers {
			request.SetHeader(header.Key, header.Value)
		}
		resp, err := request.Post(url)
		if err != nil {
			log.Errorf("request %v %v chat completions stream err: %v", url, provider, err)
			return
		} else if resp.StatusCode() >= 300 {
			log.Errorf("request %v %v chat completions stream http status %v msg: %v", url, provider, resp.StatusCode(), resp.String())
			return
		}
		scan := bufio.NewScanner(resp.RawResponse.Body)
		for scan.Scan() {
			ret <- respConverter(true, scan.Text())
		}
	}()
	return ret, nil
}
