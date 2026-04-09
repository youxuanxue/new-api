//go:build tt
// +build tt

package model

// RunPostUserCreationHooks runs TT-specific logic after a user row is persisted (password/OAuth Insert paths).
func RunPostUserCreationHooks(userId int) {
	if userId <= 0 {
		return
	}
	if DefaultTrialConfig.AutoGrant {
		_ = GrantTrialBalance(userId)
	}
}
