package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	knowledgeBase_service "github.com/UnicomAI/wanwu/api/proto/knowledgebase-service"
	rag_service "github.com/UnicomAI/wanwu/api/proto/rag-service"
	"github.com/UnicomAI/wanwu/internal/rag-service/client/model"
	"github.com/UnicomAI/wanwu/internal/rag-service/config"
	http_client "github.com/UnicomAI/wanwu/internal/rag-service/pkg/http-client"
	"github.com/UnicomAI/wanwu/pkg/log"
)

const (
	DefaultTemperature      = 0.14
	DefaultTopP             = 0.85
	DefaultFrequencyPenalty = 1.1
	DefaultTermWeight       = 1
	InitialBufferSize       = 64 * 1024        // 初始缓冲区大小：64KB
	MaxBufferCapacity       = 10 * 1024 * 1024 // 最大缓冲区容量：10MB
	MetaValueTypeNumber     = "number"
	MetaValueTypeTime       = "time"
	MetaConditionEmpty      = "empty"
	MetaConditionNotEmpty   = "not empty"
)

type RagChatParams struct {
	KnowledgeBase        []string              `json:"knowledgeBase"`
	Question             string                `json:"question"`
	Threshold            float32               `json:"threshold"` // Score阈值
	TopK                 int32                 `json:"topK"`
	Stream               bool                  `json:"stream"`
	Chichat              bool                  `json:"chichat"` // 当知识库召回结果为空时是否使用默认话术（兜底），默认为true
	RerankModelId        string                `json:"rerank_model_id"`
	CustomModelInfo      *CustomModelInfo      `json:"custom_model_info"`
	History              []*HistoryItem        `json:"history"`
	MaxHistory           int32                 `json:"max_history"`
	RewriteQuery         bool                  `json:"rewrite_query"`   // 是否query改写
	RerankMod            string                `json:"rerank_mod"`      // rerank_model:重排序模式，weighted_score：权重搜索
	RetrieveMethod       string                `json:"retrieve_method"` // hybrid_search:混合搜索， semantic_search:向量搜索， full_text_search：文本搜索
	Weight               *WeightParams         `json:"weights"`         // 权重搜索下的权重配置
	Temperature          float32               `json:"temperature"`
	TopP                 float32               `json:"top_p"`                         // 多样性
	RepetitionPenalty    float32               `json:"repetition_penalty"`            // 重复惩罚/频率惩罚
	ReturnMeta           bool                  `json:"return_meta"`                   // 是否返回元数据
	AutoCitation         bool                  `json:"auto_citation"`                 // 是否自动角标
	TermWeight           float32               `json:"term_weight_coefficient"`       // 关键词系数
	MetaFilter           bool                  `json:"metadata_filtering"`            // 元数据过滤开关
	MetaFilterConditions []*MetadataFilterItem `json:"metadata_filtering_conditions"` // 元数据过滤条件
}

type MetadataFilterItem struct {
	FilterKnowledgeName string      `json:"filtering_kb_name"`
	LogicalOperator     string      `json:"logical_operator"`
	Conditions          []*MetaItem `json:"conditions"`
}

type MetaItem struct {
	MetaName           string      `json:"meta_name"`           // 元数据名称
	MetaType           string      `json:"meta_type"`           // 元数据类型
	ComparisonOperator string      `json:"comparison_operator"` // 比较运算符
	Value              interface{} `json:"value,omitempty"`     // 用于过滤的条件值
}

type WeightParams struct {
	VectorWeight float32 `json:"vector_weight"` //语义权重
	TextWeight   float32 `json:"text_weight"`   //关键字权重
}

type CustomModelInfo struct {
	LlmModelID string `json:"llm_model_id"`
}

type HistoryItem struct {
	Query       string `json:"query"`
	Response    string `json:"response"`
	NeedHistory bool   `json:"needHistory"`
}

type ModelConfig struct {
	Temperature            float32 `json:"temperature"`
	TemperatureEnable      bool    `json:"temperatureEnable"`
	TopP                   float32 `json:"topP"`
	TopPEnable             bool    `json:"topPEnable"`
	FrequencyPenalty       float32 `json:"frequencyPenalty"`
	FrequencyPenaltyEnable bool    `json:"frequencyPenaltyEnable"`
	PresencePenalty        float32 `json:"presencePenalty"`
	PresencePenaltyEnable  bool    `json:"presencePenaltyEnable"`
}

func RagStreamChat(ctx context.Context, userId string, req *RagChatParams) (<-chan string, error) {
	params, err := buildHttpParams(userId, req)
	if err != nil {
		log.Errorf("build http params fail", err.Error())
		return nil, err
	}
	ret := make(chan string, 1024)
	go func() {
		// 确保通道最终被关闭
		defer close(ret)

		// 捕获 panic 并记录日志（不重新抛出，避免崩溃）
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("RagStreamChat panic: %v", r)
			}
		}()

		//1.开启超时监控
		if params.Timeout == 0 {
			params.Timeout = time.Minute * 10
		}
		ctx, cancel := context.WithTimeout(ctx, params.Timeout)
		defer cancel()

		resp, err := http_client.GetClient().PostJsonOriResp(ctx, params)
		if err != nil {
			errMsg := fmt.Sprintf("error: 调用下游服务异常: %v", err)
			log.Errorf(errMsg)
			ret <- errMsg
			return
		}
		defer resp.Body.Close() // 确保响应体关闭

		if resp.StatusCode != http.StatusOK {
			errMsg := fmt.Sprintf("error: 调用下游服务异常: %s", resp.Status)
			log.Errorf(errMsg)
			ret <- errMsg
			return
		}
		scan := bufio.NewScanner(resp.Body)

		//设置初始缓冲区为 64KB，最大允许 10MB
		buf := make([]byte, InitialBufferSize)
		scan.Buffer(buf, MaxBufferCapacity)

		for scan.Scan() {
			ret <- scan.Text()
		}
		if err := scan.Err(); err != nil {
			errMsg := fmt.Sprintf("error: 调用下游服务异常: %v", err)
			log.Errorf(errMsg)
			ret <- errMsg
		}
	}()

	return ret, nil
}

func buildHttpParams(userId string, req *RagChatParams) (*http_client.HttpRequestParams, error) {
	url := fmt.Sprintf("%s%s", config.Cfg().RagServer.ChatEndpoint, config.Cfg().RagServer.ChatUrl)
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return &http_client.HttpRequestParams{
		Url:        url,
		Body:       body,
		Headers:    map[string]string{"X-uid": userId},
		Timeout:    time.Minute * 10,
		MonitorKey: "rag_search_service",
		LogLevel:   http_client.LogAll,
	}, nil
}

// BuildChatConsultParams 构造rag 会话参数
func BuildChatConsultParams(req *rag_service.ChatRagReq, rag *model.RagInfo, knowledgeInfoList *knowledgeBase_service.KnowledgeDetailSelectListResp) (*RagChatParams, error) {
	// 知识库参数
	ragChatParams := &RagChatParams{}
	knowledgeConfig := rag.KnowledgeBaseConfig
	ragChatParams.MaxHistory = int32(knowledgeConfig.MaxHistory)
	ragChatParams.Threshold = float32(knowledgeConfig.Threshold)
	ragChatParams.TopK = int32(knowledgeConfig.TopK)
	ragChatParams.RetrieveMethod = buildRetrieveMethod(knowledgeConfig.MatchType)
	ragChatParams.RerankMod = buildRerankMod(knowledgeConfig.PriorityMatch)
	ragChatParams.Weight = buildWeight(knowledgeConfig)
	var kbNameList []string
	knowledgeIDToName := make(map[string]string)
	for _, v := range knowledgeInfoList.List {
		kbNameList = append(kbNameList, v.Name)
		if _, exists := knowledgeIDToName[v.KnowledgeId]; !exists {
			knowledgeIDToName[v.KnowledgeId] = v.Name
		}
	}
	ragChatParams.KnowledgeBase = kbNameList
	ragChatParams.RerankModelId = buildRerankId(knowledgeConfig.PriorityMatch, rag.RerankConfig.ModelId)
	if rag.KnowledgeBaseConfig.TermWeightEnable {
		ragChatParams.TermWeight = float32(rag.KnowledgeBaseConfig.TermWeight)
	}
	// RAG属性参数
	ragChatParams.Question = req.Question
	ragChatParams.Stream = true
	ragChatParams.Chichat = true
	ragChatParams.History = []*HistoryItem{}
	ragChatParams.RewriteQuery = true
	ragChatParams.ReturnMeta = true
	//自动角标
	ragChatParams.AutoCitation = true

	// 模型参数
	ragChatParams.CustomModelInfo = &CustomModelInfo{LlmModelID: rag.ModelConfig.ModelId}
	modelConfigStr := rag.ModelConfig.Config
	modelConfig := ModelConfig{}
	err := json.Unmarshal([]byte(modelConfigStr), &modelConfig)
	if err != nil {
		log.Errorf("model config unmarshal fail: %s", modelConfigStr)
		ragChatParams.Temperature = DefaultTemperature
		ragChatParams.TopP = DefaultTopP
		ragChatParams.RepetitionPenalty = DefaultFrequencyPenalty
		return ragChatParams, nil
	}
	if modelConfig.TemperatureEnable {
		ragChatParams.Temperature = modelConfig.Temperature
	} else {
		ragChatParams.Temperature = DefaultTemperature
	}
	if modelConfig.TopPEnable {
		ragChatParams.TopP = modelConfig.TopP
	} else {
		ragChatParams.TopP = DefaultTopP
	}
	if modelConfig.FrequencyPenaltyEnable {
		ragChatParams.RepetitionPenalty = modelConfig.FrequencyPenalty
	} else {
		ragChatParams.RepetitionPenalty = DefaultFrequencyPenalty
	}
	filterEnable, metaParams, err := buildRagMetaParams(rag, knowledgeIDToName)
	if err != nil {
		return nil, err
	}
	ragChatParams.MetaFilter = filterEnable
	ragChatParams.MetaFilterConditions = metaParams

	log.Infof("ragparams = %+v", http_client.Convert2LogString(ragChatParams))
	return ragChatParams, nil
}

func buildRagMetaParams(rag *model.RagInfo, knowledgeIDToName map[string]string) (bool, []*MetadataFilterItem, error) {
	var perKbConfig []*rag_service.RagPerKnowledgeConfig
	if rag.KnowledgeBaseConfig.MetaParams != "" {
		err := json.Unmarshal([]byte(rag.KnowledgeBaseConfig.MetaParams), &perKbConfig)
		if err != nil {
			return false, nil, errors.New("rag meta params unmarshal fail: " + err.Error())
		}
	}
	filterEnable := false // 标记是否有启用的元数据过滤
	var metaFilterConditions []*MetadataFilterItem
	for _, k := range perKbConfig {
		// 检查元数据过滤参数是否有效
		filterParams := k.RagMetaFilter
		if !isValidFilterParams(k.RagMetaFilter) {
			continue
		}
		// 校验合法值
		if k.RagMetaFilter.FilterLogicType == "" {
			return false, nil, errors.New("rag meta FilterLogicType is empty")
		}
		// 标记元数据过滤生效
		filterEnable = true
		// 构建元数据过滤条件
		metaItems, err := buildRagMetaItems(k.KnowledgeId, filterParams.FilterItems)
		if err != nil {
			return false, nil, err
		}
		// 添加过滤项到结果
		metaFilterConditions = append(metaFilterConditions, &MetadataFilterItem{
			FilterKnowledgeName: knowledgeIDToName[k.KnowledgeId],
			LogicalOperator:     filterParams.FilterLogicType,
			Conditions:          metaItems,
		})
	}
	return filterEnable, metaFilterConditions, nil
}

func isValidFilterParams(params *rag_service.RagMetaFilter) bool {
	return params != nil &&
		params.FilterEnable &&
		params.FilterItems != nil &&
		len(params.FilterItems) > 0
}

// 构建元数据项列表
func buildRagMetaItems(knowledgeID string, params []*rag_service.RagMetaFilterItem) ([]*MetaItem, error) {
	var metaItems []*MetaItem
	for _, param := range params {
		// 基础参数校验
		if err := validateMetaFilterParam(knowledgeID, param); err != nil {
			return nil, err
		}
		// 转换参数值
		ragValue, err := convertValue(param.Value, param.Type)
		if err != nil {
			log.Errorf("kbId: %s, convert value failed: %v", knowledgeID, err)
			return nil, fmt.Errorf("convert value for key %s failed: %s", param.Key, err.Error())
		}
		metaItems = append(metaItems, &MetaItem{
			MetaName:           param.Key,
			MetaType:           param.Type,
			ComparisonOperator: param.Condition,
			Value:              ragValue,
		})
	}
	return metaItems, nil
}

func convertValue(value, valueType string) (interface{}, error) {
	if len(value) == 0 {
		return nil, nil
	}
	// 根据类型转换value
	if valueType == MetaValueTypeNumber {
		ragValue, err := strconv.Atoi(value)
		if err != nil {
			log.Errorf("convertMetaValue fail %v", err)
			return nil, err
		}
		return ragValue, nil
	}
	if valueType == MetaValueTypeTime {
		parseInt, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			log.Errorf("convertMetaValue fail %v", err)
			return nil, err
		}
		return parseInt, nil
	}
	return value, nil
}

// 校验元数据过滤参数
func validateMetaFilterParam(knowledgeID string, param *rag_service.RagMetaFilterItem) error {
	// 检查关键参数是否为空
	if param.Key == "" || param.Type == "" || param.Condition == "" {
		errMsg := "key/type/condition cannot be empty"
		log.Errorf("kbId: %s, %s", knowledgeID, errMsg)
		return errors.New(errMsg)
	}

	// 检查空条件与值的匹配性
	if param.Condition == MetaConditionEmpty || param.Condition == MetaConditionNotEmpty {
		if param.Value != "" {
			errMsg := "condition is empty/non-empty, value should be empty"
			log.Errorf("kbId: %s, %s", knowledgeID, errMsg)
			return errors.New(errMsg)
		}
	} else {
		if param.Value == "" {
			errMsg := "value is empty"
			log.Errorf("kbId: %s, %s", knowledgeID, errMsg)
			return errors.New(errMsg)
		}
	}

	return nil
}

// buildRerankId 构造重排序模型id
func buildRerankId(priorityType int32, rerankId string) string {
	if priorityType == 1 {
		return ""
	}
	return rerankId
}

// buildRetrieveMethod 构造检索方式
func buildRetrieveMethod(matchType string) string {
	switch matchType {
	case "vector":
		return "semantic_search" // 向量检索
	case "text":
		return "full_text_search" // 全文检索
	case "mix":
		return "hybrid_search" // 混合检索
	}
	return ""
}

// buildRerankMod 构造重排序模式
func buildRerankMod(priorityType int32) string {
	if priorityType == 1 {
		return "weighted_score"
	}
	return "rerank_model"
}

// buildWeight 构造权重信息
func buildWeight(knowConfig model.KnowledgeBaseConfig) *WeightParams {
	if knowConfig.PriorityMatch != 1 {
		return nil
	}
	return &WeightParams{
		VectorWeight: float32(knowConfig.SemanticsPriority),
		TextWeight:   float32(knowConfig.KeywordPriority),
	}
}
