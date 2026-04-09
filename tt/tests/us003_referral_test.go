package tests

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	ttmodel "github.com/QuantumNous/new-api/model"
)

func TestUS003_ApplyReferralCode(t *testing.T) {
	inviter := &model.User{
		Username: "inviter-us003-apply",
		Email:    "inviter-us003-apply@example.com",
		AffCode:  "INVITE123",
		Status:   1,
	}
	if err := testDB.Create(inviter).Error; err != nil {
		t.Fatalf("create inviter failed: %v", err)
	}

	invitee := &model.User{
		Username: "invitee-us003-apply",
		Email:    "invitee-us003-apply@example.com",
		AffCode:  nextAffCode("US003I"),
		Status:   1,
	}
	if err := testDB.Create(invitee).Error; err != nil {
		t.Fatalf("create invitee failed: %v", err)
	}

	referral, err := ttmodel.ApplyReferralCode(int(invitee.Id), "INVITE123", "127.0.0.1")
	if err != nil {
		t.Fatalf("apply referral code failed: %v", err)
	}

	if referral.InviterId != uint(inviter.Id) {
		t.Errorf("expected inviter ID %d, got %d", inviter.Id, referral.InviterId)
	}
	if referral.InviteeId != uint(invitee.Id) {
		t.Errorf("expected invitee ID %d, got %d", invitee.Id, referral.InviteeId)
	}
	if referral.Status != "pending" {
		t.Errorf("expected pending referral status, got %s", referral.Status)
	}
	if !referral.BonusUSD.Equal(ttmodel.DefaultReferralConfig.BonusUSD) {
		t.Errorf("expected bonus %s, got %s", ttmodel.DefaultReferralConfig.BonusUSD.StringFixed(2), referral.BonusUSD.StringFixed(2))
	}
	if referral.GrantedAt != nil {
		t.Errorf("expected no granted_at when first charge is required")
	}

	t.Logf("✓ US-003: Apply referral code test passed")
}

func TestUS003_SelfReferral(t *testing.T) {
	user := &model.User{
		Username: "selfref-us003",
		Email:    "selfref-us003@example.com",
		AffCode:  "SELFREF",
		Status:   1,
	}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatalf("create self-ref user failed: %v", err)
	}

	_, err := ttmodel.ApplyReferralCode(int(user.Id), "SELFREF", "127.0.0.1")
	if err == nil {
		t.Error("Expected error for self-referral, but got none")
	}

	t.Logf("✓ US-003: Self referral prevention test passed")
}

func TestUS003_InviteeCannotApplyTwice(t *testing.T) {
	inviter := &model.User{
		Username: "inviter-us003-twice",
		Email:    "inviter-us003-twice@example.com",
		AffCode:  "INVITE-TWICE",
		Status:   1,
	}
	if err := testDB.Create(inviter).Error; err != nil {
		t.Fatalf("create inviter failed: %v", err)
	}

	invitee := &model.User{
		Username: "invitee-us003-twice",
		Email:    "invitee-us003-twice@example.com",
		AffCode:  nextAffCode("US003T"),
		Status:   1,
	}
	if err := testDB.Create(invitee).Error; err != nil {
		t.Fatalf("create invitee failed: %v", err)
	}

	if _, err := ttmodel.ApplyReferralCode(int(invitee.Id), "INVITE-TWICE", "127.0.0.2"); err != nil {
		t.Fatalf("first apply should succeed, got: %v", err)
	}

	_, err := ttmodel.ApplyReferralCode(int(invitee.Id), "INVITE-TWICE", "127.0.0.3")
	if err == nil {
		t.Fatalf("expected duplicate apply to fail")
	}

	var count int64
	if err := testDB.Model(&ttmodel.Referral{}).Where("invitee_id = ?", invitee.Id).Count(&count).Error; err != nil {
		t.Fatalf("count referrals failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 referral record for invitee, got %d", count)
	}
}
