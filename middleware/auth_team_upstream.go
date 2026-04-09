//go:build !tt
// +build !tt

package middleware

import "github.com/gin-gonic/gin"

func tryTeamAPIKeyAuth(_ *gin.Context, _ string) (handled bool, err error) {
	return false, nil
}
