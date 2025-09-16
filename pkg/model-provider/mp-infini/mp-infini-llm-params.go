package mp_infini

type LLMParams struct {
	Temperature            float32 `json:"temperature"`            // 温度
	TemperatureEnable      bool    `json:"temperatureEnable"`      // 温度(开关)
	TopP                   float32 `json:"topP"`                   // Top P
	TopPEnable             bool    `json:"topPEnable"`             // Top P(开关)
	FrequencyPenalty       float32 `json:"frequencyPenalty"`       // 频率惩罚
	FrequencyPenaltyEnable bool    `json:"frequencyPenaltyEnable"` // 频率惩罚(开关)
	PresencePenalty        float32 `json:"presencePenalty"`        // 存在惩罚
	PresencePenaltyEnable  bool    `json:"presencePenaltyEnable"`  // 存在惩罚(开关)
	MaxTokens              int32   `json:"maxTokens"`              // 最大标记
	MaxTokensEnable        bool    `json:"maxTokensEnable"`        // 最大标记(开关)
}

func (cfg *LLMParams) GetParams() map[string]interface{} {
	ret := make(map[string]interface{})

	if cfg.TemperatureEnable {
		ret["temperature"] = cfg.Temperature
	}
	if cfg.TopPEnable {
		ret["top_p"] = cfg.TopP
	}
	if cfg.FrequencyPenaltyEnable {
		ret["frequency_penalty"] = cfg.FrequencyPenalty
	}
	if cfg.PresencePenaltyEnable {
		ret["presence_penalty"] = cfg.PresencePenalty
	}
	if cfg.MaxTokensEnable {
		ret["max_tokens"] = cfg.MaxTokens
	}
	return ret
}
