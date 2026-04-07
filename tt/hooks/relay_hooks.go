// Package hooks provides TT-specific hook points for the relay flow
// This package is only included in TT builds, not in upstream PRs
package hooks

import (
	"fmt"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
)

// HookContext contains all the context needed for TT hooks
type HookContext struct {
	GinContext   *gin.Context
	Request      dto.Request
	RelayFormat  string
}

// HookResult contains the results from hook execution
type HookResult struct {
	Modified    bool
	Headers     map[string]string
	LogMessages []string
}

// RelayHooks interface for TT-specific relay hooks
type RelayHooks interface {
	// OnRequestParsed is called after request is parsed, before relay
	OnRequestParsed(ctx *HookContext) *HookResult

	// OnRelaySuccess is called after successful relay
	OnRelaySuccess(ctx *HookContext) *HookResult

	// OnRelayError is called on relay error
	OnRelayError(ctx *HookContext, err error) *HookResult
}

// TT specific hooks implementation
type TTRelayHooks struct {
	paramCorrector ParamCorrector
}

// ParamCorrector interface for parameter correction
type ParamCorrector interface {
	CorrectRequest(c *gin.Context, request dto.Request) *CorrectionResult
	GetCorrectionResult(c *gin.Context) *CorrectionResult
}

// CorrectionResult from param correction
type CorrectionResult struct {
	OriginalModel      string
	CorrectedModel     string
	OriginalMaxTokens  uint
	CorrectedMaxTokens uint
	AddedStream        bool
	IsDeprecated       bool
	IsAlias            bool
	WasAdjusted        bool
}

var (
	// Global hooks instance
	globalHooks RelayHooks
)

// InitHooks initializes TT-specific hooks
func InitHooks() {
	globalHooks = &TTRelayHooks{
		paramCorrector: NewTTParamCorrector(),
	}
}

// GetHooks returns the global hooks instance
func GetHooks() RelayHooks {
	if globalHooks == nil {
		InitHooks()
	}
	return globalHooks
}

// OnRequestParsed - hook entry point for request parsing
func OnRequestParsed(c *gin.Context, request dto.Request, relayFormat string) {
	if globalHooks == nil {
		return
	}

	ctx := &HookContext{
		GinContext:  c,
		Request:     request,
		RelayFormat: relayFormat,
	}

	result := globalHooks.OnRequestParsed(ctx)
	if result != nil && len(result.LogMessages) > 0 {
		for _, msg := range result.LogMessages {
			logger.LogInfo(c.Request.Context(), msg)
		}
	}
}

// OnRelaySuccess - hook entry point for successful relay
func OnRelaySuccess(c *gin.Context, request dto.Request) {
	if globalHooks == nil {
		return
	}

	ctx := &HookContext{
		GinContext: c,
		Request:    request,
	}

	result := globalHooks.OnRelaySuccess(ctx)
	if result != nil {
		for k, v := range result.Headers {
			c.Writer.Header().Set(k, v)
		}
	}
}

// TTRelayHooks implementation

func (h *TTRelayHooks) OnRequestParsed(ctx *HookContext) *HookResult {
	if h.paramCorrector == nil {
		return nil
	}

	h.paramCorrector.CorrectRequest(ctx.GinContext, ctx.Request)

	return &HookResult{}
}

func (h *TTRelayHooks) OnRelaySuccess(ctx *HookContext) *HookResult {
	if h.paramCorrector == nil {
		return nil
	}

	correction := h.paramCorrector.GetCorrectionResult(ctx.GinContext)
	if correction == nil || !correction.WasAdjusted {
		return nil
	}

	headers := make(map[string]string)

	if correction.IsAlias {
		headers["X-Param-Correction-Model-Alias"] =
			fmt.Sprintf("%s->%s", correction.OriginalModel, correction.CorrectedModel)
	}
	if correction.IsDeprecated {
		headers["X-Param-Correction-Deprecated"] =
			fmt.Sprintf("%s->%s", correction.OriginalModel, correction.CorrectedModel)
	}
	if correction.OriginalMaxTokens > 0 && correction.CorrectedMaxTokens > 0 {
		headers["X-Param-Correction-Max-Tokens"] =
			fmt.Sprintf("%d->%d", correction.OriginalMaxTokens, correction.CorrectedMaxTokens)
	}
	if correction.AddedStream {
		headers["X-Param-Correction-Stream"] = "auto-added"
	}

	return &HookResult{
		Modified: true,
		Headers:  headers,
	}
}

func (h *TTRelayHooks) OnRelayError(ctx *HookContext, err error) *HookResult {
	// No TT-specific error handling currently
	return nil
}

// AddContextKey adds TT-specific context keys safely
func AddContextKey(c *gin.Context, key string, value any) {
	c.Set(key, value)
}

// GetContextValue safely gets a context value
func GetContextValue(c *gin.Context, key string) (any, bool) {
	return c.Get(key)
}

func init() {
	// Auto-initialize hooks
	InitHooks()
}
