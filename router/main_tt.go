//go:build tt
// +build tt

// Package router provides HTTP routing
// main_tt.go - TT-specific router initialization (only included in TT builds)
package router

import (
	"github.com/gin-gonic/gin"
)

// initTT registers TT-specific routes
func initTT(router *gin.Engine) {
	SetTTApiRouter(router)
	SetTTAdminRouter(router)
	SetTTPublicRouter(router)
}
