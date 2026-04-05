//go:build tt
// +build tt

// Package controller provides HTTP handlers
// relay_hooks_tt.go - TT-specific hooks implementation (only included in TT builds)
package controller

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/tt/hooks"

	"github.com/gin-gonic/gin"
)

// onRequestParsedHook calls TT hooks after request parsing
func onRequestParsedHook(c *gin.Context, request dto.Request) {
	hooks.OnRequestParsed(c, request, "")
}

// onRelaySuccessHook calls TT hooks after successful relay
func onRelaySuccessHook(c *gin.Context, request dto.Request) {
	hooks.OnRelaySuccess(c, request)
}
