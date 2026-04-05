// Package controller 提供TT API控制器
// playground.go - 模型 Playground 控制器
package controller

import (
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ========== 模型 Playground（V2.0功能） ==========

// PlaygroundRequest Playground 请求
type PlaygroundRequest struct {
	Models   []string        `json:"models" binding:"required"` // 要对比的模型列表
	Messages []dto.Message   `json:"messages" binding:"required"`
	Stream   bool            `json:"stream"`
	MaxTokens *uint          `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

// PlaygroundResponse Playground 响应
type PlaygroundResponse struct {
	RequestId string                `json:"request_id"`
	Results   []PlaygroundResult    `json:"results"`
	CreatedAt string                `json:"created_at"`
}

// PlaygroundResult 单个模型的 Playground 结果
type PlaygroundResult struct {
	Model         string          `json:"model"`
	Status        string          `json:"status"` // success/failed/timeout
	Response      string          `json:"response,omitempty"`
	InputTokens   int64           `json:"input_tokens,omitempty"`
	OutputTokens  int64           `json:"output_tokens,omitempty"`
	CostUSD       string          `json:"cost_usd,omitempty"`
	LatencyMs     int64           `json:"latency_ms"`
	Error         string          `json:"error,omitempty"`
	FinishReason  string          `json:"finish_reason,omitempty"`
}

// PlaygroundSingleRequest 单模型 Playground 请求
type PlaygroundSingleRequest struct {
	Model       string        `json:"model" binding:"required"`
	Messages    []dto.Message `json:"messages" binding:"required"`
	Stream      bool          `json:"stream"`
	MaxTokens   *uint         `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

// RunPlayground 运行 Playground 对比测试
func RunPlayground(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req PlaygroundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 限制最多同时对比 4 个模型
	if len(req.Models) > 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 4 models allowed"})
		return
	}

	// 检查用户余额
	user, err := ttmodel.GetUserById(userId, false)
	if err != nil || user.Quota <= 0 {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "insufficient balance"})
		return
	}

	requestId := c.GetString(common.RequestIdKey)
	if requestId == "" {
		requestId = common.GetRandomString(16)
	}

	// 并行请求各模型
	results := make([]PlaygroundResult, len(req.Models))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, model := range req.Models {
		wg.Add(1)
		go func(idx int, modelName string) {
			defer wg.Done()

			startTime := time.Now()
			result := PlaygroundResult{
				Model:  modelName,
				Status: "success",
			}

			// TODO: 实际调用模型 API
			// 这里是简化实现，模拟响应
			result.InputTokens = 100
			result.OutputTokens = 150
			result.CostUSD = "0.001"
			result.LatencyMs = time.Since(startTime).Milliseconds()
			result.Response = "This is a simulated response from " + modelName
			result.FinishReason = "stop"

			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, model)
	}

	wg.Wait()

	c.JSON(http.StatusOK, PlaygroundResponse{
		RequestId: requestId,
		Results:   results,
		CreatedAt: time.Now().Format(time.RFC3339),
	})
}

// RunPlaygroundSingle 运行单模型 Playground
func RunPlaygroundSingle(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req PlaygroundSingleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	startTime := time.Now()

	// TODO: 实际调用模型 API
	result := PlaygroundResult{
		Model:        req.Model,
		Status:       "success",
		InputTokens:  100,
		OutputTokens: 150,
		CostUSD:      "0.001",
		LatencyMs:    time.Since(startTime).Milliseconds(),
		Response:     "This is a simulated response from " + req.Model,
		FinishReason: "stop",
	}

	c.JSON(http.StatusOK, result)
}

// PlaygroundStreamRequest Playground 流式请求
type PlaygroundStreamRequest struct {
	Model       string        `json:"model" binding:"required"`
	Messages    []dto.Message `json:"messages" binding:"required"`
	MaxTokens   *uint         `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

// RunPlaygroundStream 运行 Playground 流式响应
func RunPlaygroundStream(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req PlaygroundStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// TODO: 实际调用模型 API 并流式返回
	// 这里是简化实现，发送模拟数据
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	// 模拟流式响应
	chunks := []string{"Hello", " from", " ", req.Model, "!"}
	for _, chunk := range chunks {
		data := map[string]interface{}{
			"model": req.Model,
			"choices": []map[string]interface{}{
				{
					"delta": map[string]string{
						"content": chunk,
					},
					"finish_reason": nil,
				},
			},
		}
		c.SSEvent("message", data)
		flusher.Flush()
		time.Sleep(100 * time.Millisecond)
	}

	// 发送结束标记
	c.SSEvent("message", map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"delta":          map[string]string{},
				"finish_reason":  "stop",
			},
		},
	})
	flusher.Flush()
}

// GetPlaygroundModels 获取 Playground 可用模型列表
func GetPlaygroundModels(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	models := []map[string]interface{}{
		{
			"id":          "claude-sonnet-4-6",
			"name":        "Claude Sonnet 4.6",
			"provider":    "Anthropic",
			"category":    "chat",
			"max_tokens":  16000,
			"input_price": "$2.00/1M",
			"output_price": "$10.00/1M",
		},
		{
			"id":          "claude-opus-4-6",
			"name":        "Claude Opus 4.6",
			"provider":    "Anthropic",
			"category":    "chat",
			"max_tokens":  32000,
			"input_price": "$10.00/1M",
			"output_price": "$50.00/1M",
		},
		{
			"id":          "gpt-4o",
			"name":        "GPT-4o",
			"provider":    "OpenAI",
			"category":    "chat",
			"max_tokens":  16384,
			"input_price": "$2.50/1M",
			"output_price": "$10.00/1M",
		},
		{
			"id":          "gpt-4o-mini",
			"name":        "GPT-4o Mini",
			"provider":    "OpenAI",
			"category":    "chat",
			"max_tokens":  16384,
			"input_price": "$0.15/1M",
			"output_price": "$0.60/1M",
		},
		{
			"id":          "gemini-2.5-flash",
			"name":        "Gemini 2.5 Flash",
			"provider":    "Google",
			"category":    "chat",
			"max_tokens":  65536,
			"input_price": "$0.15/1M",
			"output_price": "$0.60/1M",
		},
		{
			"id":          "deepseek-chat",
			"name":        "DeepSeek Chat",
			"provider":    "DeepSeek",
			"category":    "chat",
			"max_tokens":  8192,
			"input_price": "$0.14/1M",
			"output_price": "$0.28/1M",
		},
		{
			"id":          "doubao-seedream-3.0",
			"name":        "Doubao Seedream 3.0",
			"provider":    "Volcengine",
			"category":    "image",
			"max_tokens":  0,
			"input_price": "$0.008/image",
			"output_price": "-",
		},
	}

	c.JSON(http.StatusOK, gin.H{"data": models})
}

// PlaygroundHistory Playground 历史记录
type PlaygroundHistory struct {
	Id          string    `json:"id"`
	UserId      uint      `json:"user_id"`
	Models      []string  `json:"models"`
	Prompt      string    `json:"prompt"`
	CreatedAt   time.Time `json:"created_at"`
}

// GetPlaygroundHistory 获取 Playground 历史记录
func GetPlaygroundHistory(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// TODO: 从数据库获取历史记录
	history := []PlaygroundHistory{}

	c.JSON(http.StatusOK, gin.H{"data": history})
}
