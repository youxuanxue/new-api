//go:build tt
// +build tt

package model

func getTTAutoMigrateModels() []interface{} {
	return []interface{}{
		&UserExtension{},
		&Referral{},
		&Plan{},
		&Subscription{},
		&ConsumptionRecord{},
		&Payment{},
		&ModelPricing{},
		&Admin{},
		&AdminAuditLog{},
		&Webhook{},
		&UserBudgetConfig{},
		&PlaygroundHistory{},
		&PoolAccount{},
		&Team{},
		&TeamMember{},
		&TeamAPIKey{},
		&SLAConfig{},
		&SLAReport{},
		&SLAIncident{},
		&SLABreach{},
		&SSOConfig{},
	}
}
