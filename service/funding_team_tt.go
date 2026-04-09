//go:build tt
// +build tt

package service

import "github.com/QuantumNous/new-api/model"

// TeamFunding bills relay requests against a team's USD balance (quota units, same scale as user quota).
type TeamFunding struct {
	teamId   uint
	consumed int
}

func (t *TeamFunding) Source() string { return BillingSourceTeam }

func (t *TeamFunding) PreConsume(amount int) error {
	if amount <= 0 {
		return nil
	}
	if err := model.PreConsumeTeamBalance(t.teamId, amount); err != nil {
		return err
	}
	t.consumed = amount
	return nil
}

func (t *TeamFunding) Settle(delta int) error {
	if delta == 0 {
		return nil
	}
	return model.DeltaTeamBalance(t.teamId, delta)
}

func (t *TeamFunding) Refund() error {
	if t.consumed <= 0 {
		return nil
	}
	return model.DeltaTeamBalance(t.teamId, -t.consumed)
}
