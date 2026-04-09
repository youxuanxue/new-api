//go:build !tt
// +build !tt

package service

import relaycommon "github.com/QuantumNous/new-api/relay/common"

func notifyBudgetAfterSettleIfApplicable(_ *relaycommon.RelayInfo) {}
