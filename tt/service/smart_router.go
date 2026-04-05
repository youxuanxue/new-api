// Package service 提供TT业务服务层
package service

import (
	"strings"
)

// SmartRouter 智能路由服务
// 根据请求特征自动选择最优模型，降低成本
type SmartRouter struct {
	// 配置
	defaultModel       string // 默认模型
	codeModel          string // 代码任务模型
	simpleQAModel      string // 简单问答模型
	longContextModel   string // 长上下文模型
	enableSmartRouting bool   // 是否启用智能路由
}

// SmartRouterConfig 智能路由配置
type SmartRouterConfig struct {
	DefaultModel       string
	CodeModel          string
	SimpleQAModel      string
	LongContextModel   string
	EnableSmartRouting bool
}

// NewSmartRouter 创建智能路由服务
func NewSmartRouter(config *SmartRouterConfig) *SmartRouter {
	if config == nil {
		config = &SmartRouterConfig{
			DefaultModel:       "claude-sonnet-4-6",
			CodeModel:          "claude-sonnet-4-6",
			SimpleQAModel:      "claude-haiku",
			LongContextModel:   "claude-sonnet-4-6",
			EnableSmartRouting: true,
		}
	}
	return &SmartRouter{
		defaultModel:       config.DefaultModel,
		codeModel:          config.CodeModel,
		simpleQAModel:      config.SimpleQAModel,
		longContextModel:   config.LongContextModel,
		enableSmartRouting: config.EnableSmartRouting,
	}
}

// RouteRequest 路由请求到最优模型
func (r *SmartRouter) RouteRequest(request *RouterRequest) string {
	if !r.enableSmartRouting {
		return r.defaultModel
	}

	// 如果用户明确指定了模型（非 auto），直接返回
	if request.Model != "" && request.Model != "auto" {
		return request.Model
	}

	// 计算任务复杂度得分
	score := r.calculateComplexityScore(request)

	// 根据得分选择模型
	return r.selectModelByScore(score, request)
}

// RouterRequest 路由请求
type RouterRequest struct {
	Model           string         // 用户请求的模型
	Messages        []RouterMessage // 消息列表
	MaxTokens       int            // 最大 token 数
	Temperature     float64        // 温度参数
	EstimatedTokens int            // 估算的 token 数
}

// RouterMessage 路由消息
type RouterMessage struct {
	Role    string
	Content string
}

// ComplexityScore 复杂度得分
type ComplexityScore struct {
	Total          int
	CodeIndicator  int // 代码相关指标
	SimpleQA       int // 简单问答指标
	LongContext    int // 长上下文指标
	Creative       int // 创意写作指标
}

// calculateComplexityScore 计算任务复杂度得分
func (r *SmartRouter) calculateComplexityScore(request *RouterRequest) *ComplexityScore {
	score := &ComplexityScore{}

	// 合并所有消息内容
	var allContent strings.Builder
	for _, msg := range request.Messages {
		allContent.WriteString(msg.Content)
		allContent.WriteString(" ")
	}
	content := allContent.String()
	contentLower := strings.ToLower(content)

	// 代码相关关键词
	codeKeywords := []string{
		"func ", "function", "class ", "import ", "from ", "def ",
		"return ", "if ", "else ", "for ", "while ", "async ",
		"await ", "const ", "let ", "var ", "interface ", "struct ",
		"package ", "git ", "commit", "merge", "pull request",
		"debug", "error", "exception", "stack trace", "代码",
		"函数", "变量", "循环", "条件", "模块", "修复", "重构",
	}

	codeCount := 0
	for _, keyword := range codeKeywords {
		codeCount += strings.Count(contentLower, keyword)
	}
	if codeCount > 0 {
		score.CodeIndicator = min(codeCount*10, 50)
	}

	// 简单问答指标
	simpleQAKeywords := []string{
		"what is", "what are", "how to", "explain", "define",
		"是什么", "什么是", "如何", "怎么", "解释", "定义",
		"简单", "介绍", "区别", "比较",
	}
	simpleQACount := 0
	for _, keyword := range simpleQAKeywords {
		simpleQACount += strings.Count(contentLower, keyword)
	}
	if simpleQACount > 0 && codeCount == 0 {
		score.SimpleQA = min(simpleQACount*15, 40)
	}

	// 长上下文指标
	if request.EstimatedTokens > 10000 || len(content) > 20000 {
		score.LongContext = 40
	} else if request.EstimatedTokens > 5000 || len(content) > 10000 {
		score.LongContext = 20
	}

	// 创意写作指标
	creativeKeywords := []string{
		"write", "create", "generate", "story", "article", "blog",
		"写作", "创作", "生成", "故事", "文章", "博客", "小说",
	}
	creativeCount := 0
	for _, keyword := range creativeKeywords {
		creativeCount += strings.Count(contentLower, keyword)
	}
	if creativeCount > 0 {
		score.Creative = min(creativeCount*10, 30)
	}

	// 计算总分
	score.Total = score.CodeIndicator + score.SimpleQA + score.LongContext + score.Creative

	return score
}

// selectModelByScore 根据得分选择模型
func (r *SmartRouter) selectModelByScore(score *ComplexityScore, request *RouterRequest) string {
	// 优先级：代码任务 > 长上下文 > 简单问答 > 创意写作 > 默认

	// 代码任务 - 使用最强的编程模型
	if score.CodeIndicator >= 20 {
		return r.codeModel
	}

	// 长上下文任务
	if score.LongContext >= 30 {
		return r.longContextModel
	}

	// 简单问答 - 使用最快最便宜的模型
	if score.SimpleQA >= 20 && score.CodeIndicator == 0 && score.Creative == 0 {
		return r.simpleQAModel
	}

	// 创意写作或复杂任务 - 使用默认模型
	return r.defaultModel
}

// GetRoutingStats 获取路由统计信息
func (r *SmartRouter) GetRoutingStats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":             r.enableSmartRouting,
		"default_model":       r.defaultModel,
		"code_model":          r.codeModel,
		"simple_qa_model":     r.simpleQAModel,
		"long_context_model":  r.longContextModel,
	}
}

// EstimateTokens 估算 token 数量（简单估算：约 4 字符 = 1 token）
func EstimateTokens(content string) int {
	return len(content) / 4
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ========== Auto 模型路由（V2.0功能） ==========

// AutoModelConfig auto 模型路由配置
type AutoModelConfig struct {
	// 编程任务
	CodeModel string `json:"code_model"`
	// 简单问答（性价比优先）
	SimpleQAModel string `json:"simple_qa_model"`
	// 长文档分析
	LongContextModel string `json:"long_context_model"`
	// 图片生成
	ImageModel string `json:"image_model"`
	// 默认模型
	DefaultModel string `json:"default_model"`
}

// DefaultAutoModelConfig 默认 auto 模型配置
var DefaultAutoModelConfig = AutoModelConfig{
	CodeModel:        "claude-sonnet-4-6",
	SimpleQAModel:    "deepseek-chat",
	LongContextModel: "gemini-2.5-flash",
	ImageModel:       "doubao-seedream-3.0",
	DefaultModel:     "claude-sonnet-4-6",
}

// currentAutoConfig 当前配置
var currentAutoConfig = DefaultAutoModelConfig

// RouteAutoModel 自动选择模型
// 当用户请求 model: "auto" 时调用
func (r *SmartRouter) RouteAutoModel(request *RouterRequest) string {
	// 检测任务类型
	taskType := r.detectTaskType(request)

	switch taskType {
	case TaskTypeCode:
		return currentAutoConfig.CodeModel
	case TaskTypeSimpleQA:
		return currentAutoConfig.SimpleQAModel
	case TaskTypeLongContext:
		return currentAutoConfig.LongContextModel
	case TaskTypeImage:
		return currentAutoConfig.ImageModel
	default:
		return currentAutoConfig.DefaultModel
	}
}

// TaskType 任务类型
type TaskType int

const (
	TaskTypeUnknown TaskType = iota
	TaskTypeCode
	TaskTypeSimpleQA
	TaskTypeLongContext
	TaskTypeImage
	TaskTypeCreative
)

// detectTaskType 检测任务类型
func (r *SmartRouter) detectTaskType(request *RouterRequest) TaskType {
	// 合并所有消息内容
	var allContent strings.Builder
	for _, msg := range request.Messages {
		allContent.WriteString(msg.Content)
		allContent.WriteString(" ")
	}
	content := allContent.String()
	contentLower := strings.ToLower(content)

	// 检测图片生成任务
	imageKeywords := []string{
		"generate image", "create image", "draw", "picture", "photo",
		"生成图片", "画图", "图片", "图像", "绘图",
		"doubao-seedream", "imagen", "dall-e",
	}
	for _, keyword := range imageKeywords {
		if strings.Contains(contentLower, keyword) {
			return TaskTypeImage
		}
	}

	// 检测代码任务
	codeKeywords := []string{
		"func ", "function", "class ", "import ", "def ",
		"return ", "if ", "else ", "for ", "while ",
		"debug", "error", "fix", "bug", "代码",
		"函数", "变量", "修复", "调试",
	}
	codeCount := 0
	for _, keyword := range codeKeywords {
		codeCount += strings.Count(contentLower, keyword)
	}
	if codeCount >= 2 {
		return TaskTypeCode
	}

	// 检测长上下文任务
	if request.EstimatedTokens > 10000 || len(content) > 20000 {
		return TaskTypeLongContext
	}

	// 检测简单问答
	simpleQAKeywords := []string{
		"what is", "how to", "explain", "define",
		"是什么", "如何", "解释", "定义",
	}
	for _, keyword := range simpleQAKeywords {
		if strings.Contains(contentLower, keyword) && len(content) < 500 {
			return TaskTypeSimpleQA
		}
	}

	return TaskTypeUnknown
}

// GetAutoModelConfig 获取 auto 模型配置
func GetAutoModelConfig() AutoModelConfig {
	return currentAutoConfig
}

// SetAutoModelConfig 设置 auto 模型配置
func SetAutoModelConfig(config AutoModelConfig) {
	currentAutoConfig = config
}

// GetAutoModelRecommendation 获取 auto 模型推荐（带解释）
func GetAutoModelRecommendation(request *RouterRequest) map[string]interface{} {
	router := NewSmartRouter(nil)
	taskType := router.detectTaskType(request)
	recommendedModel := router.RouteAutoModel(request)

	taskTypeStr := "general"
	switch taskType {
	case TaskTypeCode:
		taskTypeStr = "code"
	case TaskTypeSimpleQA:
		taskTypeStr = "simple_qa"
	case TaskTypeLongContext:
		taskTypeStr = "long_context"
	case TaskTypeImage:
		taskTypeStr = "image"
	}

	return map[string]interface{}{
		"recommended_model": recommendedModel,
		"task_type":         taskTypeStr,
		"original_model":    request.Model,
		"estimated_tokens":  request.EstimatedTokens,
	}
}
