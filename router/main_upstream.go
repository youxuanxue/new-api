//go:build !tt
// +build !tt

package router

import "github.com/gin-gonic/gin"

// initTT is a no-op in upstream builds.
func initTT(_ *gin.Engine) {}
