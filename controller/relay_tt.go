//go:build tt
// +build tt

// Package controller provides HTTP handlers
// relay_tt.go - TT-specific relay modifications (only included in TT builds)
package controller

import (
	"github.com/QuantumNous/new-api/tt/hooks"

	"github.com/gin-gonic/gin"
)

// init registers TT hooks
func init() {
	// TT hooks are auto-initialized
	hooks.InitHooks()
}
