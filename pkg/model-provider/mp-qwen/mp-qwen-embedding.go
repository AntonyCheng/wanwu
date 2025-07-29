package mp_qwen

import (
	"context"
	url_p "net/url"

	mp_common "github.com/UnicomAI/wanwu/pkg/model-provider/mp-common"
)

type Embedding struct {
	ApiKey      string `json:"apiKey"`      // ApiKey
	EndpointUrl string `json:"endpointUrl"` // 推理url
}

func (cfg *Embedding) NewReq(req *mp_common.EmbeddingReq) (mp_common.IEmbeddingReq, error) {
	m, err := req.Data()
	if err != nil {
		return nil, err
	}
	return mp_common.NewEmbeddingReq(m), nil
}

func (cfg *Embedding) Embeddings(ctx context.Context, req mp_common.IEmbeddingReq, headers ...mp_common.Header) (mp_common.IEmbeddingResp, error) {
	b, err := mp_common.Embeddings(ctx, "qwen", cfg.ApiKey, cfg.embeddingsUrl(), req.Data(), headers...)
	if err != nil {
		return nil, err
	}
	return mp_common.NewEmbeddingResp(string(b)), nil
}

func (cfg *Embedding) embeddingsUrl() string {
	ret, _ := url_p.JoinPath(cfg.EndpointUrl, "/embeddings")
	return ret
}
