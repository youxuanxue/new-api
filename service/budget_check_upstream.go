//go:build !tt
// +build !tt

package service

import (
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func checkUserBudgetBeforeBilling(_ *relaycommon.RelayInfo) *types.NewAPIError {
	return nil
}
