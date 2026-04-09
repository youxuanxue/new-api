//go:build !tt
// +build !tt

package service

import (
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func maybeTeamBillingSession(_ *gin.Context, _ *relaycommon.RelayInfo, _ int) (*BillingSession, *types.NewAPIError) {
	return nil, nil
}
