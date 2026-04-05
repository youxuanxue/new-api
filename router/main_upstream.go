//go:build !tt
// +build !tt

// Package router provides HTTP routing
// main_upstream.go - Stub for upstream builds (no TT routes)
package router

import (
	"github.com/gin-gonic/gin"
)

// initTT is a no-op in upstream builds
func initTT(router *gin.Engine) {
	// No TT routes in upstream builds
}
