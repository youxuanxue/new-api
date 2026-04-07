//go:build tt
// +build tt

package service

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type VerifyResult struct {
	Model            string `json:"model"`
	Status           string `json:"status"`
	ThinkingDetected bool   `json:"thinking_detected"`
	ResponseTime     int64  `json:"response_time_ms"`
	Message          string `json:"message,omitempty"`
}

func VerifyModelAuthenticity(modelName string) (*VerifyResult, error) {
	startTime := time.Now()
	result := &VerifyResult{Model: modelName, Status: "failed"}

	baseURL := getAPIBaseURL(modelName)
	apiKey := getAPIKey(modelName)
	if baseURL == "" || apiKey == "" {
		result.Message = "model verification failed: missing verify API config"
		result.ResponseTime = time.Since(startTime).Milliseconds()
		return result, nil
	}

	reqBody := map[string]any{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": "Please respond with exactly: 'VERIFICATION_OK'"},
		},
		"max_tokens": 50,
	}
	reqBytes, err := common.Marshal(reqBody)
	if err != nil {
		result.Message = "failed to build request"
		return result, nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/chat/completions", strings.NewReader(string(reqBytes)))
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Message = "failed to read response"
		return result, nil
	}
	if resp.StatusCode != http.StatusOK {
		result.Message = fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body))
		return result, nil
	}

	var chatResp map[string]any
	if err := common.Unmarshal(body, &chatResp); err != nil {
		result.Message = "failed to parse response"
		return result, nil
	}
	choices, ok := chatResp["choices"].([]any)
	if !ok || len(choices) == 0 {
		result.Message = "invalid response structure"
		return result, nil
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		result.Message = "invalid choice structure"
		return result, nil
	}
	message, ok := choice["message"].(map[string]any)
	if !ok {
		result.Message = "invalid message structure"
		return result, nil
	}

	content, _ := message["content"].(string)
	thinking, hasThinking := message["thinking"].(string)
	suspiciousReasons := make([]string, 0, 2)
	if content == "" && thinking == "" {
		suspiciousReasons = append(suspiciousReasons, "empty response")
	}
	if strings.Contains(strings.ToLower(modelName), "claude") && hasThinking && thinking != "" {
		result.ThinkingDetected = true
	}
	if result.ResponseTime < 50 {
		suspiciousReasons = append(suspiciousReasons, "suspiciously fast response")
	}
	if len(suspiciousReasons) > 0 {
		result.Status = "suspicious"
		result.Message = "potential issues detected: " + strings.Join(suspiciousReasons, ", ")
		return result, nil
	}
	result.Status = "verified"
	result.Message = "model verified successfully"
	return result, nil
}

func getAPIBaseURL(modelName string) string {
	if url := os.Getenv("TT_VERIFY_API_URL"); url != "" {
		return url
	}
	lower := strings.ToLower(modelName)
	if strings.Contains(lower, "claude") {
		return os.Getenv("ANTHROPIC_BASE_URL")
	}
	if strings.Contains(lower, "gpt") || strings.Contains(lower, "doubao") {
		return os.Getenv("OPENAI_BASE_URL")
	}
	return ""
}

func getAPIKey(modelName string) string {
	if key := os.Getenv("TT_VERIFY_API_KEY"); key != "" {
		return key
	}
	lower := strings.ToLower(modelName)
	if strings.Contains(lower, "claude") {
		return os.Getenv("ANTHROPIC_API_KEY")
	}
	if strings.Contains(lower, "gpt") || strings.Contains(lower, "doubao") {
		return os.Getenv("OPENAI_API_KEY")
	}
	return ""
}
