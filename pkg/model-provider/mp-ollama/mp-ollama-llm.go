package mp_ollama

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/UnicomAI/wanwu/pkg/log"
	mp_common "github.com/UnicomAI/wanwu/pkg/model-provider/mp-common"
	"github.com/UnicomAI/wanwu/pkg/util"
	"github.com/go-resty/resty/v2"
	"io"
	"net/url"
)

type LLM struct {
	ApiKey          string `json:"apiKey"`                                                           // ApiKey
	EndpointUrl     string `json:"endpointUrl"`                                                      // 推理url
	FunctionCalling string `json:"functionCalling" validate:"oneof=noSupport toolCall functionCall"` // 函数调用是否支持
}

func (cfg *LLM) NewReq(req *mp_common.LLMReq) (mp_common.ILLMReq, error) {
	m := make(map[string]interface{})
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}
	return mp_common.NewLLMReq(m), nil
}
func (cfg *LLM) ChatCompletions(ctx context.Context, req mp_common.ILLMReq, headers ...mp_common.Header) (mp_common.ILLMResp, <-chan mp_common.ILLMResp, error) {
	if cfg.ApiKey != "" {
		headers = append(headers, mp_common.Header{
			Key:   "Authorization",
			Value: "Bearer " + cfg.ApiKey,
		})
	}
	if req.Stream() {
		ret, err := chatCompletionsStream(ctx, req, cfg.chatCompletionsUrl(), headers...)
		return nil, ret, err
	}
	ret, err := chatCompletionsUnary(ctx, req, cfg.chatCompletionsUrl(), headers...)
	return ret, nil, err
}

func (cfg *LLM) chatCompletionsUrl() string {
	ret, _ := url.JoinPath(cfg.EndpointUrl, "/chat/completions")
	return ret
}

func chatCompletionsUnary(ctx context.Context, req mp_common.ILLMReq, url string, headers ...mp_common.Header) (mp_common.ILLMResp, error) {
	if req.Stream() {
		return nil, fmt.Errorf("request %v huoshan chat completions unary but stream", url)
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
		return nil, fmt.Errorf("request %v huoshan chat completions unary err: %v", url, err)
	} else if resp.StatusCode() >= 300 {
		return nil, fmt.Errorf("request %v huoshan chat completions unary http status %v msg: %v", url, resp.StatusCode(), resp.String())
	}
	b, err := io.ReadAll(resp.RawResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("request %v ollama chat completions unary read response body err: %v", url, err)
	}
	return mp_common.NewLLMResp(false, string(b)), nil
}

func chatCompletionsStream(ctx context.Context, req mp_common.ILLMReq, url string, headers ...mp_common.Header) (<-chan mp_common.ILLMResp, error) {
	if !req.Stream() {
		return nil, fmt.Errorf("request %v ollama chat completions stream but unary", url)
	}

	ret := make(chan mp_common.ILLMResp, 1024)
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
			log.Errorf("request %v ollama chat completions stream err: %v", url, err)
			return
		} else if resp.StatusCode() >= 300 {
			log.Errorf("request %v ollama chat completions stream http status %v msg: %v", url, resp.StatusCode(), resp.String())
			return
		}
		scan := bufio.NewScanner(resp.RawResponse.Body)
		for scan.Scan() {
			ret <- mp_common.NewLLMResp(true, scan.Text())
		}
	}()
	return ret, nil
}
