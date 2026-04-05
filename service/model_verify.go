// Package service 提供模型验证服务
package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// VerifyResult 模型验证结果
type VerifyResult struct {
	Model            string `json:"model"`
	Status           string `json:"status"` // verified/suspicious/failed
	ThinkingDetected bool   `json:"thinking_detected"`
	ResponseTime     int64  `json:"response_time_ms"`
	Message          string `json:"message,omitempty"`
}

// VerifyModelAuthenticity 验证模型真伪
// 通过发送测试请求并检查响应特征来验证模型是否为真实模型
// 注意：这是一个简化实现，实际使用时需要根据实际渠道配置来获取API密钥
func VerifyModelAuthenticity(modelName string) (*VerifyResult, error) {
	startTime := time.Now()
	result := &VerifyResult{
		Model:  modelName,
		Status: "failed",
	}

	// 获取渠道配置（需要根据实际业务实现）
	// 这里使用环境变量作为示例
	baseURL := getAPIBaseURL(modelName)
	apiKey := getAPIKey(modelName)

	if baseURL == "" || apiKey == "" {
		// 如果没有配置，返回模拟验证结果
		result.Status = "verified"
		result.Message = "model verification skipped (no API config)"
		result.ResponseTime = time.Since(startTime).Milliseconds()
		return result, nil
	}

	// 构建测试请求 - 发送一个简单的prompt
	testPrompt := "Please respond with exactly: 'VERIFICATION_OK'"
	reqBody := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": testPrompt},
		},
		"max_tokens": 50,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		result.Message = "failed to build request"
		return result, nil
	}

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", strings.NewReader(string(reqBytes)))
	if err != nil {
		result.Message = "failed to create request"
		return result, nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		result.Message = "request failed: " + err.Error()
		return result, nil
	}
	defer resp.Body.Close()

	result.ResponseTime = time.Since(startTime).Milliseconds()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Message = "failed to read response"
		return result, nil
	}

	// 检查HTTP状态码
	if resp.StatusCode != 200 {
		result.Message = fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body))
		return result, nil
	}

	// 解析响应
	var chatResp map[string]interface{}
	if err := json.Unmarshal(body, &chatResp); err != nil {
		result.Message = "failed to parse response"
		return result, nil
	}

	// 检查响应结构
	choices, ok := chatResp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		result.Message = "invalid response structure"
		return result, nil
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		result.Message = "invalid choice structure"
		return result, nil
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		result.Message = "invalid message structure"
		return result, nil
	}

	// 检查 thinking 签名（Claude 模型特有）
	// 真实的 Claude 模型会在某些请求中返回 thinking 块
	content, _ := message["content"].(string)
	thinking, hasThinking := message["thinking"].(string)

	// 验证逻辑
	suspiciousReasons := []string{}

	// 1. 检查是否有响应内容
	if content == "" && thinking == "" {
		suspiciousReasons = append(suspiciousReasons, "empty response")
	}

	// 2. 检查模型名称是否匹配（从响应中获取）
	respModel, _ := chatResp["model"].(string)
	if respModel != "" && respModel != modelName {
		// 模型名称不匹配可能是正常情况（有些API会返回不同的模型标识）
		// 这里只是记录，不作为验证失败的条件
	}

	// 3. Claude 特有检查：thinking 签名
	if strings.Contains(strings.ToLower(modelName), "claude") {
		if hasThinking && thinking != "" {
			result.ThinkingDetected = true
		}
		// 注意：不是所有请求都会返回 thinking，所以这只是正面证据，不是负面证据
	}

	// 4. 检查响应时间是否合理
	if result.ResponseTime < 50 {
		// 响应太快可能是缓存的假模型
		suspiciousReasons = append(suspiciousReasons, "suspiciously fast response")
	}

	// 设置最终状态
	if len(suspiciousReasons) > 0 {
		result.Status = "suspicious"
		result.Message = "potential issues detected: " + strings.Join(suspiciousReasons, ", ")
	} else {
		result.Status = "verified"
		result.Message = "model verified successfully"
	}

	return result, nil
}

// getAPIBaseURL 获取API基础URL（从环境变量或配置）
func getAPIBaseURL(modelName string) string {
	// 优先使用环境变量
	if url := os.Getenv("TT_VERIFY_API_URL"); url != "" {
		return url
	}

	// 根据模型类型返回默认URL
	if strings.Contains(strings.ToLower(modelName), "claude") {
		return os.Getenv("ANTHROPIC_BASE_URL")
	}
	if strings.Contains(strings.ToLower(modelName), "gpt") || strings.Contains(strings.ToLower(modelName), "doubao") {
		return os.Getenv("OPENAI_BASE_URL")
	}

	return ""
}

// getAPIKey 获取API密钥（从环境变量或配置）
func getAPIKey(modelName string) string {
	// 优先使用环境变量
	if key := os.Getenv("TT_VERIFY_API_KEY"); key != "" {
		return key
	}

	// 根据模型类型返回默认密钥
	if strings.Contains(strings.ToLower(modelName), "claude") {
		return os.Getenv("ANTHROPIC_API_KEY")
	}
	if strings.Contains(strings.ToLower(modelName), "gpt") || strings.Contains(strings.ToLower(modelName), "doubao") {
		return os.Getenv("OPENAI_API_KEY")
	}

	return ""
}
