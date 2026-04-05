// Package middleware 提供TT核心中间件
// semantic_cache.go - 语义缓存中间件，相似请求复用响应
package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
)

// SemanticCacheConfig 语义缓存配置
type SemanticCacheConfig struct {
	// Enabled 是否启用语义缓存
	Enabled bool
	// TTL 缓存过期时间（秒）
	TTL int
	// MaxCacheSize 最大缓存条目数
	MaxCacheSize int
	// SimilarityThreshold 相似度阈值（0-1）
	SimilarityThreshold float64
	// EnableExactCache 是否启用精确匹配缓存
	EnableExactCache bool
	// ExcludeModels 排除的模型列表（不缓存的模型）
	ExcludeModels []string
	// MinInputTokens 最小输入 token 数（小于此值不缓存）
	MinInputTokens int
	// EmbeddingModel 用于计算嵌入的模型
	EmbeddingModel string
}

// DefaultSemanticCacheConfig 默认配置
var DefaultSemanticCacheConfig = SemanticCacheConfig{
	Enabled:             true,
	TTL:                 3600, // 1 小时
	MaxCacheSize:        10000,
	SimilarityThreshold: 0.95,
	EnableExactCache:    true,
	ExcludeModels: []string{
		"o1", "o1-mini", "o1-preview",
		"deepseek-reasoner",
	},
	MinInputTokens:   100,
	EmbeddingModel:   "text-embedding-3-small",
}

// currentCacheConfig 当前配置
var currentCacheConfig = DefaultSemanticCacheConfig

// CacheEntry 缓存条目
type CacheEntry struct {
	Key         string          `json:"key"`
	Model       string          `json:"model"`
	RequestHash string          `json:"request_hash"`
	Response    json.RawMessage `json:"response"`
	Usage       *dto.Usage      `json:"usage"`
	CreatedAt   time.Time       `json:"created_at"`
	ExpiresAt   time.Time       `json:"expires_at"`
	HitCount    int             `json:"hit_count"`
	Embedding   []float32       `json:"embedding,omitempty"` // 请求嵌入向量
	Messages    string          `json:"messages"`            // 用于相似度比较的消息摘要
}

// SemanticCache 语义缓存
type SemanticCache struct {
	config    SemanticCacheConfig
	entries   map[string]*CacheEntry
	mu        sync.RWMutex
	hitCount  int64
	missCount int64
}

// globalSemanticCache 全局语义缓存实例
var globalSemanticCache *SemanticCache
var cacheOnce sync.Once

// GetSemanticCache 获取全局语义缓存实例
func GetSemanticCache() *SemanticCache {
	cacheOnce.Do(func() {
		globalSemanticCache = &SemanticCache{
			config:  currentCacheConfig,
			entries: make(map[string]*CacheEntry),
		}
	})
	return globalSemanticCache
}

// CacheStats 缓存统计
type CacheStats struct {
	TotalEntries int64   `json:"total_entries"`
	HitCount     int64   `json:"hit_count"`
	MissCount    int64   `json:"miss_count"`
	HitRate      float64 `json:"hit_rate"`
	TotalSaved   int64   `json:"total_saved_tokens"`
}

// GetCacheKey 生成缓存键
func GetCacheKey(model string, messages []dto.Message, temperature float64) string {
	// 构建消息摘要
	var sb strings.Builder
	sb.WriteString(model)
	sb.WriteString(":")
	for _, msg := range messages {
		sb.WriteString(msg.Role)
		sb.WriteString(":")
		sb.WriteString(getContentString(msg.Content))
		sb.WriteString(";")
	}
	// 添加温度参数（分组）
	tempGroup := int(temperature * 10)
	sb.WriteString("temp:")
	sb.WriteString(string(rune('0' + tempGroup)))

	// 计算哈希
	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:])
}

// getContentString 从 content 中提取字符串
func getContentString(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []any:
		var sb strings.Builder
		for _, item := range c {
			if m, ok := item.(map[string]any); ok {
				if text, ok := m["text"].(string); ok {
					sb.WriteString(text)
				}
			}
		}
		return sb.String()
	default:
		return ""
	}
}

// GetRequestHash 生成请求哈希（用于精确匹配）
func GetRequestHash(request *dto.GeneralOpenAIRequest) string {
	data, err := json.Marshal(map[string]any{
		"model":       request.Model,
		"messages":    request.Messages,
		"temperature": request.Temperature,
		"max_tokens":  request.MaxTokens,
	})
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ShouldCache 判断是否应该缓存请求
func ShouldCache(c *gin.Context, request *dto.GeneralOpenAIRequest) bool {
	if !currentCacheConfig.Enabled {
		return false
	}

	// 检查模型是否在排除列表中
	for _, excluded := range currentCacheConfig.ExcludeModels {
		if strings.HasPrefix(request.Model, excluded) {
			return false
		}
	}

	// 检查是否是流式请求（流式请求也可以缓存，但需要特殊处理）
	// 流式请求缓存时存储完整响应

	// 检查消息长度
	msgLen := 0
	for _, msg := range request.Messages {
		msgLen += len(getContentString(msg.Content))
	}
	if msgLen < currentCacheConfig.MinInputTokens*4 { // 粗略估计 token 数
		return false
	}

	return true
}

// GetCachedResponse 获取缓存的响应
func (sc *SemanticCache) GetCachedResponse(c *gin.Context, request *dto.GeneralOpenAIRequest) (*CacheEntry, bool) {
	if !sc.config.Enabled {
		return nil, false
	}

	sc.mu.RLock()
	defer sc.mu.RUnlock()

	// 精确匹配缓存
	if sc.config.EnableExactCache {
		requestHash := GetRequestHash(request)
		for _, entry := range sc.entries {
			if entry.RequestHash == requestHash && !entry.ExpiresAt.Before(time.Now()) {
				entry.HitCount++
				sc.hitCount++
				logger.LogInfo(c, "[SemanticCache] Exact match hit for model %s", request.Model)
				return entry, true
			}
		}
	}

	// TODO: 语义相似度匹配
	// 需要实现向量嵌入和相似度计算
	// 当前版本仅支持精确匹配

	sc.missCount++
	return nil, false
}

// SetCachedResponse 设置缓存响应
func (sc *SemanticCache) SetCachedResponse(c *gin.Context, request *dto.GeneralOpenAIRequest, response json.RawMessage, usage *dto.Usage) {
	if !sc.config.Enabled {
		return
	}

	if !ShouldCache(c, request) {
		return
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	// 检查缓存大小限制
	if len(sc.entries) >= sc.config.MaxCacheSize {
		// 清理过期条目
		sc.cleanupExpired()
		// 如果仍然超出限制，删除最旧的条目
		if len(sc.entries) >= sc.config.MaxCacheSize {
			sc.evictOldest()
		}
	}

	entry := &CacheEntry{
		Key:         GetCacheKey(request.Model, request.Messages, float64Value(request.Temperature)),
		Model:       request.Model,
		RequestHash: GetRequestHash(request),
		Response:    response,
		Usage:       usage,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(time.Duration(sc.config.TTL) * time.Second),
		HitCount:    0,
	}

	sc.entries[entry.Key] = entry
	logger.LogInfo(c, "[SemanticCache] Cached response for model %s", request.Model)
}

// cleanupExpired 清理过期条目
func (sc *SemanticCache) cleanupExpired() {
	now := time.Now()
	for key, entry := range sc.entries {
		if entry.ExpiresAt.Before(now) {
			delete(sc.entries, key)
		}
	}
}

// evictOldest 删除最旧的条目
func (sc *SemanticCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range sc.entries {
		if first || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
			first = false
		}
	}

	if oldestKey != "" {
		delete(sc.entries, oldestKey)
	}
}

// GetStats 获取缓存统计
func (sc *SemanticCache) GetStats() CacheStats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	total := sc.hitCount + sc.missCount
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(sc.hitCount) / float64(total)
	}

	return CacheStats{
		TotalEntries: int64(len(sc.entries)),
		HitCount:     sc.hitCount,
		MissCount:    sc.missCount,
		HitRate:      hitRate,
	}
}

// Clear 清空缓存
func (sc *SemanticCache) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.entries = make(map[string]*CacheEntry)
	sc.hitCount = 0
	sc.missCount = 0
}

// float64Value 安全获取 float64 指针的值
func float64Value(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// SemanticCacheMiddleware 语义缓存中间件
func SemanticCacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只处理 POST 请求
		if c.Request.Method != "POST" {
			c.Next()
			return
		}

		// 只处理 chat completions 请求
		path := c.Request.URL.Path
		if !strings.HasSuffix(path, "/chat/completions") {
			c.Next()
			return
		}

		c.Next()
	}
}

// SetSemanticCacheConfig 设置语义缓存配置
func SetSemanticCacheConfig(config SemanticCacheConfig) {
	currentCacheConfig = config
	if globalSemanticCache != nil {
		globalSemanticCache.config = config
	}
}

// GetSemanticCacheConfig 获取当前配置
func GetSemanticCacheConfig() SemanticCacheConfig {
	return currentCacheConfig
}

// GetCacheStatsAPI 获取缓存统计 API
func GetCacheStatsAPI(c *gin.Context) {
	cache := GetSemanticCache()
	stats := cache.GetStats()

	c.JSON(200, gin.H{
		"enabled":        currentCacheConfig.Enabled,
		"total_entries":  stats.TotalEntries,
		"hit_count":      stats.HitCount,
		"miss_count":     stats.MissCount,
		"hit_rate":       stats.HitRate,
		"ttl_seconds":    currentCacheConfig.TTL,
		"max_cache_size": currentCacheConfig.MaxCacheSize,
	})
}

// ClearCacheAPI 清空缓存 API
func ClearCacheAPI(c *gin.Context) {
	cache := GetSemanticCache()
	cache.Clear()

	c.JSON(200, gin.H{
		"success": true,
		"message": "Cache cleared",
	})
}

// init 初始化
func init() {
	// 从环境变量读取配置
	if common.GetEnvOrDefault("TT_SEMANTIC_CACHE_ENABLED", "true") == "false" {
		currentCacheConfig.Enabled = false
	}
}
