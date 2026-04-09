//go:build tt
// +build tt

package service

import (
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/QuantumNous/new-api/model"
)

// notifyBudgetAfterSettleIfApplicable sends threshold webhooks after a billed request settles.
func notifyBudgetAfterSettleIfApplicable(relayInfo *relaycommon.RelayInfo) {
	if relayInfo == nil || relayInfo.UserId <= 0 {
		return
	}
	if relayInfo.TeamId > 0 {
		return
	}
	_ = model.CheckAndSendBudgetAlert(uint(relayInfo.UserId))
}
