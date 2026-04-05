//go:build !tt
// +build !tt

// Package controller provides HTTP handlers
// relay_upstream.go - Stub for upstream builds (no TT hooks)
package controller

import (
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
)

// onRequestParsedHook is a no-op stub for upstream builds
func onRequestParsedHook(c *gin.Context, request dto.Request) {
	// No-op in upstream builds
}

// onRelaySuccessHook is a no-op stub for upstream builds
func onRelaySuccessHook(c *gin.Context, request dto.Request) {
	// No-op in upstream builds
}
