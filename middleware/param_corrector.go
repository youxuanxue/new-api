// Package middleware 提供TT核心中间件
// param_corrector.go - 参数自动纠错中间件
package middleware

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
)

// ModelLimits 定义模型的最大 token 限制
var ModelLimits = map[string]uint{
	// Claude 系列
	"claude-opus-4-6":        32000,
	"claude-sonnet-4-6":      16000,
	"claude-sonnet-4":        16000,
	"claude-haiku-3-5":       8000,
	"claude-3-5-haiku":       8000,
	"claude-3-haiku":         4096,
	"claude-3-opus":          4096,
	"claude-3-sonnet":        4096,
	// OpenAI GPT 系列
	"gpt-4o":                 16384,
	"gpt-4o-mini":            16384,
	"gpt-4-turbo":            4096,
	"gpt-4":                  4096,
	"gpt-4-32k":              32768,
	"gpt-3.5-turbo":          4096,
	"gpt-3.5-turbo-16k":      16384,
	"o1":                     100000,
	"o1-mini":                65536,
	"o1-preview":             32768,
	// Gemini 系列
	"gemini-2.5-pro":         65536,
	"gemini-2.5-flash":       65536,
	"gemini-2.0-flash":       8192,
	"gemini-1.5-pro":         8192,
	"gemini-1.5-flash":       8192,
	// DeepSeek 系列
	"deepseek-chat":          8192,
	"deepseek-reasoner":      8192,
	// 豆包系列
	"doubao-seed-1.6":        4096,
	"doubao-seedream-3.0":    4096,
	// 默认值
	"default":                4096,
}

// ModelAliases 定义模型名称别名映射（容错映射）
// 当用户请求别名时，自动映射到标准名称
var ModelAliases = map[string]string{
	// Claude 别名
	"claude-opus":            "claude-opus-4-6",
	"claude-sonnet":          "claude-sonnet-4-6",
	"claude-sonnet-4":        "claude-sonnet-4-6",
	"claude-haiku":           "claude-haiku-3-5",
	"claude-3.5-sonnet":      "claude-sonnet-4-6",
	"claude-3-5-sonnet":      "claude-sonnet-4-6",
	"claude-3.5-haiku":       "claude-haiku-3-5",
	// OpenAI 别名
	"gpt4":                   "gpt-4",
	"gpt4o":                  "gpt-4o",
	"gpt-4o-2024-11-20":      "gpt-4o",
	"gpt-4o-2024-08-06":      "gpt-4o",
	"gpt-4o-2024-05-13":      "gpt-4o",
	"gpt-4-turbo-preview":    "gpt-4-turbo",
	"gpt-4-0125-preview":     "gpt-4-turbo",
	"gpt-4-1106-preview":     "gpt-4-turbo",
	"gpt-3.5":                "gpt-3.5-turbo",
	"gpt35":                  "gpt-3.5-turbo",
	"o1-preview-2024-12-17":  "o1-preview",
	// Gemini 别名
	"gemini-pro":             "gemini-2.5-flash",
	"gemini-flash":           "gemini-2.5-flash",
	"gemini-2.0-flash-lite":  "gemini-2.0-flash",
	// DeepSeek 别名
	"deepseek":               "deepseek-chat",
	"deepseek-v3":            "deepseek-chat",
	"deepseek-r1":            "deepseek-reasoner",
}

// DeprecatedModels 定义弃用模型映射
// 请求弃用模型时自动转发到替代模型
var DeprecatedModels = map[string]string{
	// Claude 弃用模型
	"claude-2":               "claude-sonnet-4-6",
	"claude-2.1":             "claude-sonnet-4-6",
	"claude-2.0":             "claude-sonnet-4-6",
	"claude-instant-1":       "claude-haiku-3-5",
	"claude-3-sonnet":        "claude-sonnet-4-6",
	"claude-3-opus":          "claude-opus-4-6",
	// OpenAI 弃用模型
	"gpt-4-0314":             "gpt-4-turbo",
	"gpt-4-0613":             "gpt-4-turbo",
	"gpt-3.5-turbo-0301":     "gpt-3.5-turbo",
	"gpt-3.5-turbo-0613":     "gpt-3.5-turbo",
	"gpt-3.5-turbo-instruct": "gpt-3.5-turbo",
	// Gemini 弃用模型
	"gemini-1.0-pro":         "gemini-2.5-flash",
}

// CorrectionResult 记录纠错结果
type CorrectionResult struct {
	OriginalModel     string // 原始模型名
	CorrectedModel    string // 纠错后模型名
	OriginalMaxTokens uint   // 原始 max_tokens
	CorrectedMaxTokens uint   // 纠错后 max_tokens
	AddedStream       bool   // 是否添加了 stream 参数
	IsDeprecated      bool   // 是否是弃用模型
	IsAlias           bool   // 是否是别名
	WasAdjusted       bool   // 是否有任何调整
}

// ParamCorrectorConfig 参数纠错器配置
type ParamCorrectorConfig struct {
	// EnableModelAlias 是否启用模型别名纠错
	EnableModelAlias bool
	// EnableMaxTokensCorrection 是否启用 max_tokens 纠错
	EnableMaxTokensCorrection bool
	// EnableDeprecatedForward 是否启用弃用模型转发
	EnableDeprecatedForward bool
	// EnableStreamAutoComplete 是否启用 stream 自动补全（针对 Claude Code）
	EnableStreamAutoComplete bool
	// ClaudeCodeUserAgents 需要自动补全 stream 的 User-Agent 列表
	ClaudeCodeUserAgents []string
}

// DefaultParamCorrectorConfig 默认配置
var DefaultParamCorrectorConfig = ParamCorrectorConfig{
	EnableModelAlias:          true,
	EnableMaxTokensCorrection: true,
	EnableDeprecatedForward:   true,
	EnableStreamAutoComplete:  true,
	ClaudeCodeUserAgents: []string{
		"Claude Code",
		"claude-code",
		"claude_cli",
	},
}

// currentCorrectorConfig 当前配置
var currentCorrectorConfig = DefaultParamCorrectorConfig

// ParamCorrector 参数纠错中间件
func ParamCorrector() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只处理 POST 请求
		if c.Request.Method != "POST" {
			c.Next()
			return
		}

		// 只处理 API 请求
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/v1/") {
			c.Next()
			return
		}

		// 从上下文获取请求对象（由之前的中间件解析）
		// 这里的纠错会在后续处理中进行
		c.Next()
	}
}

// CorrectRequest 对请求进行参数纠错
func CorrectRequest(c *gin.Context, request dto.Request) *CorrectionResult {
	result := &CorrectionResult{
		WasAdjusted: false,
	}

	// 获取模型名称
	modelName := getModelName(request)
	if modelName == "" {
		return result
	}
	result.OriginalModel = modelName

	// 1. 模型别名纠错
	if currentCorrectorConfig.EnableModelAlias {
		if correctedModel, ok := correctModelAlias(modelName); ok {
			result.CorrectedModel = correctedModel
			result.IsAlias = true
			result.WasAdjusted = true
			request.SetModelName(correctedModel)
			modelName = correctedModel
			logger.LogInfo(c, "[ParamCorrector] Model alias corrected: %s -> %s", result.OriginalModel, correctedModel)
		}
	}

	// 2. 弃用模型转发
	if currentCorrectorConfig.EnableDeprecatedForward {
		if replacement, ok := DeprecatedModels[modelName]; ok {
			result.CorrectedModel = replacement
			result.IsDeprecated = true
			result.WasAdjusted = true
			request.SetModelName(replacement)
			modelName = replacement
			logger.LogInfo(c, "[ParamCorrector] Deprecated model forwarded: %s -> %s", result.OriginalModel, replacement)
		}
	}

	// 3. max_tokens 纠错
	if currentCorrectorConfig.EnableMaxTokensCorrection {
		maxTokens := getMaxTokens(request)
		result.OriginalMaxTokens = maxTokens
		limit := getModelMaxTokens(modelName)
		if maxTokens > limit && limit > 0 {
			result.CorrectedMaxTokens = limit
			result.WasAdjusted = true
			setMaxTokens(request, limit)
			logger.LogInfo(c, "[ParamCorrector] max_tokens corrected: %d -> %d (model limit)", maxTokens, limit)
		}
	}

	// 4. stream 自动补全（针对 Claude Code）
	if currentCorrectorConfig.EnableStreamAutoComplete {
		if shouldAutoCompleteStream(c, request) {
			result.AddedStream = true
			result.WasAdjusted = true
			setStream(request, true)
			logger.LogInfo(c, "[ParamCorrector] stream auto-completed to true for Claude Code")
		}
	}

	// 将纠错结果存储到上下文
	if result.WasAdjusted {
		c.Set(constant.ContextKeyParamCorrection, result)
	}

	return result
}

// getModelName 从请求中获取模型名称
func getModelName(request dto.Request) string {
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		return r.Model
	case *dto.ClaudeRequest:
		return r.Model
	case *dto.ImageRequest:
		return r.Model
	case *dto.AudioRequest:
		return r.Model
	case *dto.EmbeddingRequest:
		return r.Model
	case *dto.RerankRequest:
		return r.Model
	case *dto.GeminiChatRequest:
		return r.Model
	}
	return ""
}

// getMaxTokens 从请求中获取 max_tokens
func getMaxTokens(request dto.Request) uint {
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		if r.MaxCompletionTokens != nil && *r.MaxCompletionTokens > 0 {
			return *r.MaxCompletionTokens
		}
		if r.MaxTokens != nil {
			return *r.MaxTokens
		}
	case *dto.ClaudeRequest:
		if r.MaxTokens != nil {
			return uint(*r.MaxTokens)
		}
	}
	return 0
}

// setMaxTokens 设置 max_tokens
func setMaxTokens(request dto.Request, value uint) {
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		r.MaxTokens = common.GetPointer(value)
	case *dto.ClaudeRequest:
		intVal := int(value)
		r.MaxTokens = &intVal
	}
}

// setStream 设置 stream 参数
func setStream(request dto.Request, value bool) {
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		r.Stream = &value
	case *dto.ClaudeRequest:
		// Claude 的 stream 通过不同的字段处理
		// 在 Claude adapter 中会处理
	}
}

// correctModelAlias 纠正模型别名
func correctModelAlias(modelName string) (string, bool) {
	// 精确匹配
	if alias, ok := ModelAliases[modelName]; ok {
		return alias, true
	}

	// 尝试不区分大小写匹配
	lowerModel := strings.ToLower(modelName)
	for alias, target := range ModelAliases {
		if strings.ToLower(alias) == lowerModel {
			return target, true
		}
	}

	// 尝试模糊匹配（去除版本号后缀）
	// 例如 claude-sonnet-4-6-20250514 -> claude-sonnet-4-6
	if idx := strings.LastIndex(modelName, "-20"); idx > 0 {
		baseName := modelName[:idx]
		if alias, ok := ModelAliases[baseName]; ok {
			return alias, true
		}
	}

	return modelName, false
}

// getModelMaxTokens 获取模型的最大 token 限制
func getModelMaxTokens(modelName string) uint {
	// 精确匹配
	if limit, ok := ModelLimits[modelName]; ok {
		return limit
	}

	// 模糊匹配（处理带日期后缀的模型名）
	for model, limit := range ModelLimits {
		if strings.HasPrefix(modelName, model) {
			return limit
		}
	}

	// 返回默认值
	return ModelLimits["default"]
}

// shouldAutoCompleteStream 判断是否应该自动补全 stream 参数
func shouldAutoCompleteStream(c *gin.Context, request dto.Request) bool {
	// 检查是否已经有 stream 参数
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		if r.Stream != nil {
			return false
		}
	case *dto.ClaudeRequest:
		// Claude 请求的 stream 检查
		return false
	default:
		return false
	}

	// 检查 User-Agent 是否来自 Claude Code
	userAgent := c.Request.Header.Get("User-Agent")
	if userAgent == "" {
		return false
	}

	for _, pattern := range currentCorrectorConfig.ClaudeCodeUserAgents {
		if strings.Contains(userAgent, pattern) {
			return true
		}
	}

	return false
}

// GetCorrectionResult 从上下文获取纠错结果
func GetCorrectionResult(c *gin.Context) *CorrectionResult {
	if result, exists := c.Get(constant.ContextKeyParamCorrection); exists {
		if correction, ok := result.(*CorrectionResult); ok {
			return correction
		}
	}
	return nil
}

// SetParamCorrectorConfig 设置参数纠错器配置
func SetParamCorrectorConfig(config ParamCorrectorConfig) {
	currentCorrectorConfig = config
}

// GetParamCorrectorConfig 获取当前配置
func GetParamCorrectorConfig() ParamCorrectorConfig {
	return currentCorrectorConfig
}
