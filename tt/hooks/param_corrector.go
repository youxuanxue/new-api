// Package hooks provides TT-specific hook implementations
package hooks

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
)

// Context key for param correction
const ContextKeyParamCorrection = "tt_param_correction"

// TTParamCorrector implements TT-specific parameter correction
type TTParamCorrector struct {
	modelLimits    map[string]uint
	modelAliases   map[string]string
	deprecated     map[string]string
	claudeCodeUAs  []string
}

// NewTTParamCorrector creates a new parameter corrector
func NewTTParamCorrector() *TTParamCorrector {
	return &TTParamCorrector{
		modelLimits:   defaultModelLimits,
		modelAliases:  defaultModelAliases,
		deprecated:    defaultDeprecated,
		claudeCodeUAs: defaultClaudeCodeUAs,
	}
}

// Default configurations
var (
	defaultModelLimits = map[string]uint{
		"claude-opus-4-6":    32000,
		"claude-sonnet-4-6":  16000,
		"claude-haiku-3-5":   8000,
		"gpt-4o":             16384,
		"gpt-4-turbo":        4096,
		"deepseek-chat":      8192,
		"deepseek-reasoner":  8192,
	}

	defaultModelAliases = map[string]string{
		"claude-opus":     "claude-opus-4-6",
		"claude-sonnet":   "claude-sonnet-4-6",
		"claude-haiku":    "claude-haiku-3-5",
		"gpt4":            "gpt-4",
		"gpt4o":           "gpt-4o",
		"deepseek":        "deepseek-chat",
	}

	defaultDeprecated = map[string]string{
		"claude-2":         "claude-sonnet-4-6",
		"claude-2.1":       "claude-sonnet-4-6",
		"gpt-4-0314":       "gpt-4-turbo",
		"gpt-3.5-turbo-instruct": "gpt-3.5-turbo",
	}

	defaultClaudeCodeUAs = []string{
		"Claude Code",
		"claude-code",
		"claude_cli",
	}
)

// CorrectRequest applies TT-specific parameter corrections
func (pc *TTParamCorrector) CorrectRequest(c *gin.Context, request dto.Request) *CorrectionResult {
	result := &CorrectionResult{}

	modelName := pc.getModelName(request)
	if modelName == "" {
		return result
	}
	result.OriginalModel = modelName

	// 1. Deprecated model forwarding
	if replacement, ok := pc.deprecated[strings.ToLower(modelName)]; ok {
		result.CorrectedModel = replacement
		result.IsDeprecated = true
		result.WasAdjusted = true
		request.SetModelName(replacement)
		modelName = replacement
		logger.LogInfo(c, "[TTParamCorrector] Deprecated model: %s -> %s", result.OriginalModel, replacement)
	}

	// 2. Model alias resolution
	if alias, ok := pc.modelAliases[strings.ToLower(modelName)]; ok {
		result.CorrectedModel = alias
		result.IsAlias = true
		result.WasAdjusted = true
		request.SetModelName(alias)
		modelName = alias
		logger.LogInfo(c, "[TTParamCorrector] Alias resolved: %s -> %s", result.OriginalModel, alias)
	}

	// 3. max_tokens correction (TT cost control)
	maxTokens := pc.getMaxTokens(request)
	result.OriginalMaxTokens = maxTokens
	if limit, ok := pc.modelLimits[modelName]; ok {
		if maxTokens > limit {
			result.CorrectedMaxTokens = limit
			result.WasAdjusted = true
			pc.setMaxTokens(request, limit)
			logger.LogInfo(c, "[TTParamCorrector] max_tokens capped: %d -> %d", maxTokens, limit)
		}
	}

	// 4. Stream auto-complete for Claude Code
	if pc.shouldAutoCompleteStream(c, request) {
		result.AddedStream = true
		result.WasAdjusted = true
		pc.setStream(request, true)
		logger.LogInfo(c, "[TTParamCorrector] stream auto-completed for Claude Code")
	}

	// Store result in context
	if result.WasAdjusted {
		c.Set(ContextKeyParamCorrection, result)
	}

	return result
}

// GetCorrectionResult retrieves the correction result from context
func (pc *TTParamCorrector) GetCorrectionResult(c *gin.Context) *CorrectionResult {
	if val, exists := c.Get(ContextKeyParamCorrection); exists {
		if result, ok := val.(*CorrectionResult); ok {
			return result
		}
	}
	return nil
}

// Helper methods

func (pc *TTParamCorrector) getModelName(request dto.Request) string {
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
	default:
		return ""
	}
}

func (pc *TTParamCorrector) getMaxTokens(request dto.Request) uint {
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

func (pc *TTParamCorrector) setMaxTokens(request dto.Request, value uint) {
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		r.MaxTokens = common.GetPointer(value)
	case *dto.ClaudeRequest:
		intVal := int(value)
		r.MaxTokens = &intVal
	}
}

func (pc *TTParamCorrector) shouldAutoCompleteStream(c *gin.Context, request dto.Request) bool {
	// Check if stream is already set
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		if r.Stream != nil {
			return false
		}
	default:
		return false
	}

	// Check User-Agent
	userAgent := c.Request.Header.Get("User-Agent")
	if userAgent == "" {
		return false
	}

	for _, pattern := range pc.claudeCodeUAs {
		if strings.Contains(userAgent, pattern) {
			return true
		}
	}

	return false
}

func (pc *TTParamCorrector) setStream(request dto.Request, value bool) {
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		r.Stream = &value
	}
}
