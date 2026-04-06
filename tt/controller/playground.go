// Package controller 提供TT API控制器
// playground.go - 模型 Playground 控制器
package controller

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// ========== 模型 Playground（V2.0功能） ==========

// PlaygroundRequest Playground 请求
type PlaygroundRequest struct {
	Models      []string      `json:"models" binding:"required"` // 要对比的模型列表
	Messages    []dto.Message `json:"messages" binding:"required"`
	Stream      bool          `json:"stream"`
	MaxTokens   *uint         `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

// PlaygroundResponse Playground 响应
type PlaygroundResponse struct {
	RequestId string             `json:"request_id"`
	Results   []PlaygroundResult `json:"results"`
	CreatedAt string             `json:"created_at"`
}

// PlaygroundResult 单个模型的 Playground 结果
type PlaygroundResult struct {
	Model        string  `json:"model"`
	Status       string  `json:"status"` // success/failed/timeout
	Response     string  `json:"response,omitempty"`
	InputTokens  int64   `json:"input_tokens,omitempty"`
	OutputTokens int64   `json:"output_tokens,omitempty"`
	CostUSD      string  `json:"cost_usd,omitempty"`
	LatencyMs    int64   `json:"latency_ms"`
	Error        string  `json:"error,omitempty"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

// PlaygroundSingleRequest 单模型 Playground 请求
type PlaygroundSingleRequest struct {
	Model       string        `json:"model" binding:"required"`
	Messages    []dto.Message `json:"messages" binding:"required"`
	Stream      bool          `json:"stream"`
	MaxTokens   *uint         `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

// callModelAPI 调用上游模型 API
func callModelAPI(c *gin.Context, modelName string, messages []dto.Message, maxTokens *uint, temperature *float64, isStream bool) (*PlaygroundResult, error) {
	userId := c.GetInt("id")
	if userId == 0 {
		return nil, fmt.Errorf("unauthorized")
	}

	// 获取用户信息和分组
	user, err := ttmodel.GetUserById(userId, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// 获取适合模型的渠道
	channel, err := ttmodel.GetChannel(user.Group, modelName, 0)
	if err != nil || channel == nil {
		return nil, fmt.Errorf("no available channel for model %s", modelName)
	}

	// 构建请求
	req := &dto.GeneralOpenAIRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   isStream,
	}

	if maxTokens != nil {
		req.MaxTokens = maxTokens
	}
	if temperature != nil {
		req.Temperature = temperature
	}

	// 获取渠道的 API 密钥
	apiKey, _, err := channel.GetNextEnabledKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// 构建上游 URL
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		return nil, fmt.Errorf("channel has no base URL")
	}
	upstreamURL := fmt.Sprintf("%s/v1/chat/completions", strings.TrimSuffix(baseURL, "/"))

	// 序列化请求体
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), "POST", upstreamURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	// 获取 HTTP 客户端
	var httpClient *http.Client
	proxy := ""
	if channel.Setting != nil && *channel.Setting != "" {
		var channelSettings dto.ChannelSettings
		if err := json.Unmarshal([]byte(*channel.Setting), &channelSettings); err == nil {
			proxy = channelSettings.Proxy
		}
	}
	if proxy != "" {
		httpClient, err = service.NewProxyHttpClient(proxy)
		if err != nil {
			httpClient = service.GetHttpClient()
		}
	} else {
		httpClient = service.GetHttpClient()
	}

	// 发送请求
	startTime := time.Now()
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	latencyMs := time.Since(startTime).Milliseconds()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 解析 OpenAI 响应
	var openaiResp dto.OpenAITextResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 构建结果
	result := &PlaygroundResult{
		Model:     modelName,
		Status:    "success",
		LatencyMs: latencyMs,
	}

	if resp.StatusCode != http.StatusOK {
		result.Status = "failed"
		result.Error = fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(respBody))
		return result, nil
	}

	// 提取响应内容
	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]
		if choice.Message != nil {
			result.Response = choice.Message.StringContent()
		}
		result.FinishReason = choice.FinishReason
	}

	// 提取 usage 信息
	if openaiResp.Usage != nil {
		result.InputTokens = openaiResp.Usage.PromptTokens
		result.OutputTokens = openaiResp.Usage.CompletionTokens

		// 使用实际的定价计算成本
		cost, err := ttmodel.CalculateCost(modelName, result.InputTokens, result.OutputTokens)
		if err != nil {
			cost = "0.00"
		}
		result.CostUSD = cost
	}

	return result, nil
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

			// 调用模型 API
			apiResult, err := callModelAPI(c, modelName, req.Messages, req.MaxTokens, req.Temperature, req.Stream)
			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				result.LatencyMs = time.Since(startTime).Milliseconds()
			} else {
				result = *apiResult
			}

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

	// 调用模型 API
	result, err := callModelAPI(c, req.Model, req.Messages, req.MaxTokens, req.Temperature, req.Stream)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"model": req.Model,
			"status": "failed",
			"error": err.Error(),
		})
		return
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

	// 获取用户信息和分组
	user, err := ttmodel.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	// 获取适合模型的渠道
	channel, err := ttmodel.GetChannel(user.Group, req.Model, 0)
	if err != nil || channel == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("no available channel for model %s", req.Model)})
		return
	}

	// 获取渠道的 API 密钥
	apiKey, _, err := channel.GetNextEnabledKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get API key"})
		return
	}

	// 构建上游 URL
	baseURL := channel.GetBaseURL()
	upstreamURL := fmt.Sprintf("%s/v1/chat/completions", strings.TrimSuffix(baseURL, "/"))

	// 构建请求体
	reqBody := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}
	if req.MaxTokens != nil {
		reqBody["max_tokens"] = *req.MaxTokens
	}
	if req.Temperature != nil {
		reqBody["temperature"] = *req.Temperature
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal request"})
		return
	}

	// 创建上游请求
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), "POST", upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	httpReq.Header.Set("Accept", "text/event-stream")

	// 获取 HTTP 客户端
	var httpClient *http.Client
	proxy := ""
	if channel.Setting != nil && *channel.Setting != "" {
		var channelSettings dto.ChannelSettings
		if err := json.Unmarshal([]byte(*channel.Setting), &channelSettings); err == nil {
			proxy = channelSettings.Proxy
		}
	}
	if proxy != "" {
		httpClient, err = service.NewProxyHttpClient(proxy)
		if err != nil {
			httpClient = service.GetHttpClient()
		}
	} else {
		httpClient = service.GetHttpClient()
	}

	// 发送请求
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to upstream"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, gin.H{"error": string(body)})
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// 流式传输响应
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.LogError(c, "stream read error: "+err.Error())
			break
		}

		// 写入响应
		c.Writer.WriteString(line)
		flusher.Flush()

		// 检查是否是结束标记
		if strings.HasPrefix(line, "data: [DONE]") {
			break
		}
	}
}

// PlaygroundModelInfo Playground 模型信息
type PlaygroundModelInfo struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Category     string `json:"category"`
	MaxTokens    int    `json:"max_tokens"`
	InputPrice   string `json:"input_price"`
	OutputPrice  string `json:"output_price"`
}

// GetPlaygroundModels 获取 Playground 可用模型列表
func GetPlaygroundModels(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 获取用户分组
	user, err := ttmodel.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	// 获取该分组可用的模型列表
	models := ttmodel.GetGroupEnabledModels(user.Group)

	// 构建模型信息列表
	modelInfos := make([]PlaygroundModelInfo, 0, len(models))

	// 模型元数据映射 (简化实现)
	modelMetaMap := map[string]struct {
		Name        string
		Provider    string
		Category    string
		MaxTokens   int
		InputPrice  string
		OutputPrice string
	}{
		"claude-opus-4-6":    {"Claude Opus 4.6", "Anthropic", "chat", 32000, "$10.00/1M", "$50.00/1M"},
		"claude-sonnet-4-6":  {"Claude Sonnet 4.6", "Anthropic", "chat", 16000, "$2.00/1M", "$10.00/1M"},
		"claude-haiku-3-5":   {"Claude Haiku 3.5", "Anthropic", "chat", 8000, "$0.40/1M", "$2.00/1M"},
		"gpt-4o":             {"GPT-4o", "OpenAI", "chat", 16384, "$2.50/1M", "$10.00/1M"},
		"gpt-4o-mini":        {"GPT-4o Mini", "OpenAI", "chat", 16384, "$0.15/1M", "$0.60/1M"},
		"gpt-4-turbo":        {"GPT-4 Turbo", "OpenAI", "chat", 4096, "$10.00/1M", "$30.00/1M"},
		"gemini-2.5-flash":    {"Gemini 2.5 Flash", "Google", "chat", 65536, "$0.15/1M", "$0.60/1M"},
		"gemini-2.5-pro":     {"Gemini 2.5 Pro", "Google", "chat", 65536, "$1.25/1M", "$5.00/1M"},
		"deepseek-chat":      {"DeepSeek Chat", "DeepSeek", "chat", 8192, "$0.14/1M", "$0.28/1M"},
		"deepseek-reasoner":  {"DeepSeek Reasoner", "DeepSeek", "chat", 8192, "$0.55/1M", "$2.19/1M"},
		"doubao-seedream-3.0": {"Doubao Seedream 3.0", "Volcengine", "image", 0, "$0.008/image", "-"},
	}

	for _, model := range models {
		meta, ok := modelMetaMap[model]
		if !ok {
			// 默认元数据
			meta = struct {
				Name        string
				Provider    string
				Category    string
				MaxTokens   int
				InputPrice  string
				OutputPrice string
			}{
				Name:        model,
				Provider:    "Unknown",
				Category:    "chat",
				MaxTokens:   4096,
				InputPrice:  "-",
				OutputPrice: "-",
			}
		}
		modelInfos = append(modelInfos, PlaygroundModelInfo{
			Id:          model,
			Name:        meta.Name,
			Provider:    meta.Provider,
			Category:    meta.Category,
			MaxTokens:   meta.MaxTokens,
			InputPrice:  meta.InputPrice,
			OutputPrice: meta.OutputPrice,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": modelInfos})
}

// PlaygroundHistory Playground 历史记录
type PlaygroundHistory struct {
	Id        string    `json:"id"`
	UserId    uint      `json:"user_id"`
	Models    []string  `json:"models"`
	Prompt    string    `json:"prompt"`
	CreatedAt time.Time `json:"created_at"`
}

// GetPlaygroundHistory 获取 Playground 历史记录
func GetPlaygroundHistory(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 从数据库获取历史记录
	limit := 50 // 默认返回最近50条
	histories, err := ttmodel.GetPlaygroundHistory(uint(userId), limit)
	if err != nil {
		logger.LogError(c, "Failed to get playground history: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get history"})
		return
	}

	// 转换为响应格式
	result := make([]PlaygroundHistoryResponse, len(histories))
	for i, h := range histories {
		var models []string
		json.Unmarshal([]byte(h.Models), &models)
		result[i] = PlaygroundHistoryResponse{
			Id:        fmt.Sprintf("%d", h.Id),
			UserId:    h.UserId,
			Models:    models,
			Prompt:    h.Prompt,
			CreatedAt: h.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// SavePlaygroundHistory 保存 Playground 历史记录
type SavePlaygroundHistoryRequest struct {
	Models []string      `json:"models"`
	Prompt string        `json:"prompt"`
	Result PlaygroundResult `json:"result"`
}

// PlaygroundResult 单次 Playground 结果
type PlaygroundResult struct {
	Model        string `json:"model"`
	Response      string `json:"response"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CostUSD      string `json:"cost_usd"`
}

// SavePlaygroundHistory 保存 Playground 历史记录到数据库
func SavePlaygroundHistory(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req SavePlaygroundHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 构建历史记录
	modelsJson, _ := json.Marshal(req.Models)
	responseJson, _ := json.Marshal(req.Result)

	history := &ttmodel.PlaygroundHistory{
		UserId:   uint(userId),
		Models:   string(modelsJson),
		Prompt:   req.Prompt,
		Response: string(responseJson),
		CostUSD:  req.Result.CostUSD,
	}

	// 保存到数据库
	err := ttmodel.CreatePlaygroundHistory(history)
	if err != nil {
		logger.LogError(c, "Failed to save playground history: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"id":      fmt.Sprintf("%d", history.Id),
	})
}
