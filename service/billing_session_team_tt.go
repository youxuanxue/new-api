//go:build tt
// +build tt

package service

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func maybeTeamBillingSession(c *gin.Context, relayInfo *relaycommon.RelayInfo, preConsumedQuota int) (*BillingSession, *types.NewAPIError) {
	if relayInfo == nil || relayInfo.TeamId <= 0 {
		return nil, nil
	}
	teamId := uint(relayInfo.TeamId)
	spendable, err := model.GetTeamSpendableQuotaUnits(teamId)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	if spendable <= 0 {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("团队额度不足, 剩余额度: %s", logger.FormatQuota(spendable)),
			types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
			types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	if preConsumedQuota > 0 && spendable-preConsumedQuota < 0 {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("团队预扣费失败, 剩余额度: %s, 需要: %s", logger.FormatQuota(spendable), logger.FormatQuota(preConsumedQuota)),
			types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
			types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	relayInfo.UserQuota = spendable

	session := &BillingSession{
		relayInfo: relayInfo,
		funding:   &TeamFunding{teamId: teamId},
	}
	if apiErr := session.preConsume(c, preConsumedQuota); apiErr != nil {
		return nil, apiErr
	}
	return session, nil
}
