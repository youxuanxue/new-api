//go:build tt
// +build tt

package middleware

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// LogLevel 日志级别定义
type LogLevel int

const (
	// LevelSilent 极致安全，仅记录错误
	LevelSilent LogLevel = 0
	// LevelMeta 生产默认，仅记录脱敏元数据
	LevelMeta LogLevel = 1
	// LevelDebug 排障模式，临时开启
	LevelDebug LogLevel = 2
	// LevelFull 完整日志，仅开发环境
	LevelFull LogLevel = 3
)

// SecurityConfig 安全代理配置
type SecurityConfig struct {
	LogLevel         LogLevel
	MaxRequestBodyMB int64 // 请求体最大大小（MB）
	DebugAutoReset   time.Duration // Debug模式自动重置时间
}

// DefaultSecurityConfig 默认配置
var DefaultSecurityConfig = SecurityConfig{
	LogLevel:         LevelMeta, // 生产默认 Level 1
	MaxRequestBodyMB: 10,
	DebugAutoReset:   24 * time.Hour,
}

// currentLogLevel 当前日志级别（运行时可配置）
var (
	currentLogLevel  LogLevel
	logLevelMutex    sync.RWMutex
	debugResetTimer  *time.Timer
	debugLogFile     *os.File
	auditLogFile     *os.File
)

// RequestMeta 请求元数据（Level 1 记录内容）
type RequestMeta struct {
	RequestID   string    `json:"request_id"`
	Timestamp   time.Time `json:"timestamp"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	Model       string    `json:"model,omitempty"`
	UserID      string    `json:"user_id,omitempty"` // 哈希后
	ClientType  string    `json:"client_type,omitempty"`
	InputTokens int64     `json:"input_tokens,omitempty"`
	OutputTokens int64    `json:"output_tokens,omitempty"`
	CostUSD     float64   `json:"cost_usd,omitempty"`
	LatencyMs   int64     `json:"latency_ms"`
	StatusCode  int       `json:"status_code"`
	ChannelID   string    `json:"channel_id,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// DebugLogEntry 调试日志条目（Level 2 记录内容）
type DebugLogEntry struct {
	RequestMeta
	RequestHeaders  map[string]string `json:"request_headers,omitempty"`
	RequestBody     string            `json:"request_body,omitempty"` // 前1KB
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseBody    string            `json:"response_body,omitempty"` // 前1KB
	UpstreamChannel string            `json:"upstream_channel,omitempty"`
	FailoverLog     string            `json:"failover_log,omitempty"`
}

// InitSecurityProxy 初始化安全代理
func InitSecurityProxy() error {
	// 从环境变量读取日志级别
	levelStr := os.Getenv("TT_LOG_LEVEL")
	currentLogLevel = parseLogLevel(levelStr)

	// 生产环境禁止 Level 3
	if currentLogLevel == LevelFull && os.Getenv("GIN_MODE") == "release" {
		return fmt.Errorf("LevelFull not allowed in production")
	}

	// 初始化审计日志
	var err error
	auditLogFile, err = os.OpenFile("/var/log/tt/audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		log.Printf("[WARN] Failed to open audit log: %v", err)
	}

	// 初始化调试日志（如果需要）
	if currentLogLevel == LevelDebug {
		if err := initDebugLog(); err != nil {
			log.Printf("[WARN] Failed to init debug log: %v", err)
		}
	}

	log.Printf("[INFO] Security proxy initialized, log level: %d", currentLogLevel)
	return nil
}

// parseLogLevel 解析日志级别
func parseLogLevel(s string) LogLevel {
	switch strings.ToUpper(s) {
	case "SILENT", "0":
		return LevelSilent
	case "META", "1", "":
		return LevelMeta // 默认
	case "DEBUG", "2":
		return LevelDebug
	case "FULL", "3":
		return LevelFull
	default:
		return LevelMeta
	}
}

// initDebugLog 初始化调试日志
func initDebugLog() error {
	var err error
	debugLogFile, err = os.OpenFile("/var/log/tt/debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}

	// 设置自动重置定时器
	if debugResetTimer != nil {
		debugResetTimer.Stop()
	}
	debugResetTimer = time.AfterFunc(DefaultSecurityConfig.DebugAutoReset, func() {
		logLevelMutex.Lock()
		currentLogLevel = LevelMeta
		logLevelMutex.Unlock()
		log.Printf("[INFO] Debug log auto-reset to META after 24h")
	})

	return nil
}

// SetLogLevel 动态设置日志级别
func SetLogLevel(level LogLevel, operator string) error {
	logLevelMutex.Lock()
	defer logLevelMutex.Unlock()

	// 生产环境禁止 Level 3
	if level == LevelFull && os.Getenv("GIN_MODE") == "release" {
		return fmt.Errorf("LevelFull not allowed in production")
	}

	oldLevel := currentLogLevel
	currentLogLevel = level

	// 记录审计日志
	auditLog("[AUDIT] log_level_changed from=%d to=%d by=%s", oldLevel, level, operator)

	// 如果切换到 Debug，初始化调试日志并设置自动重置
	if level == LevelDebug {
		if err := initDebugLog(); err != nil {
			return err
		}
	} else if debugResetTimer != nil {
		debugResetTimer.Stop()
	}

	log.Printf("[INFO] Log level changed: %d -> %d", oldLevel, level)
	return nil
}

// GetLogLevel 获取当前日志级别
func GetLogLevel() LogLevel {
	logLevelMutex.RLock()
	defer logLevelMutex.RUnlock()
	return currentLogLevel
}

// SecurityProxy 安全代理中间件
func SecurityProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		level := GetLogLevel()

		// 读取请求体（限制大小）
		maxBytes := DefaultSecurityConfig.MaxRequestBodyMB << 20
		c.Request.Body = io.NopCloser(io.LimitReader(c.Request.Body, maxBytes))

		// 包装响应写入器以捕获状态码和响应体（Debug模式）
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		if level >= LevelDebug {
			c.Writer = blw
		}

		// 提取请求元数据
		meta := extractRequestMeta(c, start)

		// 处理请求
		c.Next()

		// 计算延迟
		meta.LatencyMs = time.Since(start).Milliseconds()
		meta.StatusCode = c.Writer.Status()

		// 从上下文获取额外信息
		if model, exists := c.Get("model"); exists {
			meta.Model = model.(string)
		}
		if inputTokens, exists := c.Get("input_tokens"); exists {
			meta.InputTokens = inputTokens.(int64)
		}
		if outputTokens, exists := c.Get("output_tokens"); exists {
			meta.OutputTokens = outputTokens.(int64)
		}
		if costUSD, exists := c.Get("cost_usd"); exists {
			meta.CostUSD = costUSD.(float64)
		}
		if channelID, exists := c.Get("channel_id"); exists {
			meta.ChannelID = channelID.(string)
		}
		if errStr, exists := c.Get("error"); exists {
			meta.Error = errStr.(string)
		}

		// 根据级别记录日志
		switch level {
		case LevelSilent:
			// 仅记录 5xx 错误
			if c.Writer.Status() >= 500 {
				log.Printf("[ERROR] request_id=%s status=%d error=%s", meta.RequestID, meta.StatusCode, meta.Error)
			}

		case LevelMeta:
			// 记录脱敏元数据
			logMeta(meta)

		case LevelDebug:
			// 记录调试信息
			logDebug(c, meta, blw)

		case LevelFull:
			// 完整日志（仅开发环境）
			logFull(c, meta, blw)
		}

		// 清理内存引用
		c.Set("request_body", nil)
		c.Set("response_body", nil)
	}
}

// extractRequestMeta 提取请求元数据
func extractRequestMeta(c *gin.Context, start time.Time) RequestMeta {
	meta := RequestMeta{
		RequestID:  c.GetHeader("X-Request-ID"),
		Timestamp:  start,
		Method:     c.Request.Method,
		Path:       c.Request.URL.Path,
		ClientType: c.GetHeader("User-Agent"),
	}

	// 用户ID哈希（脱敏）
	if userID, exists := c.Get("user_id"); exists {
		meta.UserID = hashUserID(fmt.Sprintf("%v", userID))
	}

	return meta
}

// hashUserID 用户ID哈希（脱敏）
func hashUserID(id string) string {
	if len(id) < 8 {
		return "****"
	}
	return id[:4] + "****" + id[len(id)-4:]
}

// logMeta 记录元数据日志
func logMeta(meta RequestMeta) {
	data, err := common.Marshal(meta)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal meta: %v", err)
		return
	}

	// 写入审计日志
	if auditLogFile != nil {
		auditLogFile.WriteString(string(data) + "\n")
	}

	// 同时输出到标准日志
	log.Printf("[META] %s", string(data))
}

// logDebug 记录调试日志
func logDebug(c *gin.Context, meta RequestMeta, blw *bodyLogWriter) {
	entry := DebugLogEntry{
		RequestMeta: meta,
	}

	// 提取请求头（排除敏感信息）
	entry.RequestHeaders = make(map[string]string)
	for k, v := range c.Request.Header {
		if isSensitiveHeader(k) {
			entry.RequestHeaders[k] = "****"
		} else {
			entry.RequestHeaders[k] = strings.Join(v, ", ")
		}
	}

	// 请求体前1KB
	if body, exists := c.Get("request_body"); exists {
		if bodyStr, ok := body.(string); ok {
			if len(bodyStr) > 1024 {
				entry.RequestBody = bodyStr[:1024] + "..."
			} else {
				entry.RequestBody = bodyStr
			}
		}
	}

	// 响应体前1KB
	if blw != nil && blw.body.Len() > 0 {
		respBody := blw.body.String()
		if len(respBody) > 1024 {
			entry.ResponseBody = respBody[:1024] + "..."
		} else {
			entry.ResponseBody = respBody
		}
	}

	// 上游渠道信息
	if channel, exists := c.Get("upstream_channel"); exists {
		entry.UpstreamChannel = channel.(string)
	}

	data, err := common.Marshal(entry)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal debug entry: %v", err)
		return
	}

	// 写入调试日志文件
	if debugLogFile != nil {
		debugLogFile.WriteString(string(data) + "\n")
	}

	log.Printf("[DEBUG] %s", string(data))
}

// logFull 完整日志（仅开发环境）
func logFull(c *gin.Context, meta RequestMeta, blw *bodyLogWriter) {
	// 完整请求体
	if body, err := io.ReadAll(c.Request.Body); err == nil {
		c.Set("request_body", string(body))
	}

	logDebug(c, meta, blw)
}

// isSensitiveHeader 判断是否敏感头
func isSensitiveHeader(key string) bool {
	lowerKey := strings.ToLower(key)
	sensitiveHeaders := []string{
		"authorization",
		"x-api-key",
		"cookie",
		"set-cookie",
	}
	for _, h := range sensitiveHeaders {
		if strings.Contains(lowerKey, h) {
			return true
		}
	}
	return false
}

// auditLog 审计日志
func auditLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format(time.RFC3339)
	line := fmt.Sprintf("[%s] %s\n", timestamp, msg)

	if auditLogFile != nil {
		auditLogFile.WriteString(line)
	}
	log.Printf("[AUDIT] %s", msg)
}

// bodyLogWriter 响应体日志写入器
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// ClaudeCodeCompatibility Claude Code 兼容性处理
func ClaudeCodeCompatibility() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 识别 Claude Code 客户端
		userAgent := c.GetHeader("User-Agent")
		isClaudeCode := strings.Contains(userAgent, "claude-code") ||
			strings.Contains(userAgent, "Claude Code")

		if isClaudeCode {
			c.Set("client_type", "claude_code")

			// 透传 anthropic-beta 头
			betaHeaders := c.Request.Header.Values("anthropic-beta")
			if len(betaHeaders) > 0 {
				c.Set("anthropic_beta", betaHeaders)
			}

			// 透传 anthropic-version 头
			version := c.GetHeader("anthropic-version")
			if version != "" {
				c.Set("anthropic_version", version)
			}

			// 透传 Session-Id
			sessionID := c.GetHeader("X-Claude-Code-Session-Id")
			if sessionID != "" {
				c.Set("session_id", sessionID)
			}

			// 透传 client-request-id
			clientReqID := c.GetHeader("x-client-request-id")
			if clientReqID != "" {
				c.Set("client_request_id", clientReqID)
			}
		}

		c.Next()
	}
}

// SSEKeepAlive SSE 心跳保活（Claude Code 90s 超时）
func SSEKeepAlive() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 仅对 SSE 流式请求生效
		if !strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
			c.Next()
			return
		}

		// 设置 SSE 响应头
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no") // 禁用 nginx 缓冲

		// 创建心跳上下文
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()

		// 启动心跳 goroutine
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// 发送 SSE 注释作为心跳
					c.Writer.Write([]byte(": heartbeat\n\n"))
					c.Writer.Flush()
				}
			}
		}()

		c.Next()
	}
}

// RequestIDMiddleware 请求ID中间件
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d_%s", time.Now().UnixNano(), randomString(8))
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// StreamingResponseHandler 流式响应处理器
type StreamingResponseHandler struct {
	writer    *bufio.Writer
	flusher   http.Flusher
	done      chan struct{}
}

// WriteSSEChunk 写入 SSE 数据块
func (h *StreamingResponseHandler) WriteSSEChunk(data string) error {
	_, err := h.writer.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
	if err != nil {
		return err
	}
	h.writer.Flush()
	h.flusher.Flush()
	return nil
}

// WriteSSEEvent 写入 SSE 事件
func (h *StreamingResponseHandler) WriteSSEEvent(event, data string) error {
	_, err := h.writer.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)))
	if err != nil {
		return err
	}
	h.writer.Flush()
	h.flusher.Flush()
	return nil
}

// Complete 完成流式响应
func (h *StreamingResponseHandler) Complete() {
	close(h.done)
}
