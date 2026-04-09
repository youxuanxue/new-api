//go:build tt
// +build tt

package service

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// checkUserBudgetBeforeBilling rejects relay when user-configured daily/monthly USD caps are already exceeded.
func checkUserBudgetBeforeBilling(relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	if relayInfo == nil || relayInfo.UserId <= 0 {
		return nil
	}
	if relayInfo.TeamId > 0 {
		return nil
	}
	status, err := model.GetBudgetStatus(uint(relayInfo.UserId))
	if err != nil {
		// Fail open: avoid blocking all traffic on transient DB issues.
		return nil
	}
	if status.DailyExceeded {
		model.TriggerWebhook("budget_exceeded", map[string]interface{}{
			"user_id":        relayInfo.UserId,
			"scope":          "daily",
			"daily_used":     status.DailyUsed,
			"daily_limit":    status.DailyLimit,
			"monthly_used":   status.MonthlyUsed,
			"monthly_limit":  status.MonthlyLimit,
		})
		return types.NewOpenAIError(
			errors.New("daily budget limit exceeded"),
			types.ErrorCodeBudgetExceededDaily,
			http.StatusForbidden,
		)
	}
	if status.MonthlyExceeded {
		model.TriggerWebhook("budget_exceeded", map[string]interface{}{
			"user_id":        relayInfo.UserId,
			"scope":          "monthly",
			"daily_used":     status.DailyUsed,
			"daily_limit":    status.DailyLimit,
			"monthly_used":   status.MonthlyUsed,
			"monthly_limit":  status.MonthlyLimit,
		})
		return types.NewOpenAIError(
			errors.New("monthly budget limit exceeded"),
			types.ErrorCodeBudgetExceededMonthly,
			http.StatusForbidden,
		)
	}
	return nil
}
