package tests

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

func TestUS004_ViewReferralRecordsScopedAndOrdered(t *testing.T) {
	inviter := &model.User{
		Username: "us004-inviter-" + nextAffCode("U"),
		Email:    "us004-inviter-" + nextAffCode("E") + "@example.com",
		AffCode:  "US004-CODE",
		Status:   1,
	}
	if err := testDB.Create(inviter).Error; err != nil {
		t.Fatalf("create inviter failed: %v", err)
	}

	otherInviter := &model.User{
		Username: "us004-other-inviter-" + nextAffCode("U"),
		Email:    "us004-other-inviter-" + nextAffCode("E") + "@example.com",
		AffCode:  nextAffCode("US004OTHER"),
		Status:   1,
	}
	if err := testDB.Create(otherInviter).Error; err != nil {
		t.Fatalf("create other inviter failed: %v", err)
	}

	inviteeA := &model.User{Username: "us004-invitee-a-" + nextAffCode("U"), Email: "us004-invitee-a-" + nextAffCode("E") + "@example.com", AffCode: nextAffCode("US004A"), Status: 1}
	inviteeB := &model.User{Username: "us004-invitee-b-" + nextAffCode("U"), Email: "us004-invitee-b-" + nextAffCode("E") + "@example.com", AffCode: nextAffCode("US004B"), Status: 1}
	inviteeC := &model.User{Username: "us004-invitee-c-" + nextAffCode("U"), Email: "us004-invitee-c-" + nextAffCode("E") + "@example.com", AffCode: nextAffCode("US004C"), Status: 1}
	if err := testDB.Create(inviteeA).Error; err != nil {
		t.Fatalf("create inviteeA failed: %v", err)
	}
	if err := testDB.Create(inviteeB).Error; err != nil {
		t.Fatalf("create inviteeB failed: %v", err)
	}
	if err := testDB.Create(inviteeC).Error; err != nil {
		t.Fatalf("create inviteeC failed: %v", err)
	}

	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)

	r1 := ttmodel.Referral{
		InviterId:    uint(inviter.Id),
		InviteeId:    uint(inviteeA.Id),
		InviteCode:   inviter.AffCode,
		Status:       "pending",
		BonusUSD:     ttmodel.DefaultReferralConfig.BonusUSD,
		InviterBonus: ttmodel.DefaultReferralConfig.BonusUSD,
		InviteeBonus: ttmodel.DefaultReferralConfig.BonusUSD,
		CreatedAt:    older,
	}
	r2 := ttmodel.Referral{
		InviterId:    uint(inviter.Id),
		InviteeId:    uint(inviteeB.Id),
		InviteCode:   inviter.AffCode,
		Status:       "granted",
		BonusUSD:     ttmodel.DefaultReferralConfig.BonusUSD,
		InviterBonus: ttmodel.DefaultReferralConfig.BonusUSD,
		InviteeBonus: ttmodel.DefaultReferralConfig.BonusUSD,
		CreatedAt:    newer,
	}
	rOther := ttmodel.Referral{
		InviterId:    uint(otherInviter.Id),
		InviteeId:    uint(inviteeC.Id),
		InviteCode:   otherInviter.AffCode,
		Status:       "pending",
		BonusUSD:     ttmodel.DefaultReferralConfig.BonusUSD,
		InviterBonus: ttmodel.DefaultReferralConfig.BonusUSD,
		InviteeBonus: ttmodel.DefaultReferralConfig.BonusUSD,
		CreatedAt:    time.Now(),
	}

	if err := testDB.Create(&r1).Error; err != nil {
		t.Fatalf("create r1 failed: %v", err)
	}
	if err := testDB.Create(&r2).Error; err != nil {
		t.Fatalf("create r2 failed: %v", err)
	}
	if err := testDB.Create(&rOther).Error; err != nil {
		t.Fatalf("create rOther failed: %v", err)
	}

	records, err := ttmodel.GetReferralRecords(int(inviter.Id))
	if err != nil {
		t.Fatalf("GetReferralRecords failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records for inviter %d, got %d", inviter.Id, len(records))
	}
	if records[0].InviteeId != uint(inviteeB.Id) || records[1].InviteeId != uint(inviteeA.Id) {
		t.Fatalf("expected descending order by created_at for inviter records")
	}
	for _, rec := range records {
		if rec.InviterId != uint(inviter.Id) {
			t.Fatalf("unexpected cross-user record returned: inviter_id=%d", rec.InviterId)
		}
	}
}

func TestUS004_ViewReferralRecordsEmptyForNoInvites(t *testing.T) {
	user := &model.User{
		Username: "us004-empty-" + nextAffCode("U"),
		Email:    "us004-empty-" + nextAffCode("E") + "@example.com",
		AffCode:  nextAffCode("US004EMPTY"),
		Status:   1,
	}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	records, err := ttmodel.GetReferralRecords(int(user.Id))
	if err != nil {
		t.Fatalf("GetReferralRecords failed for empty case: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records for user without invites, got %d", len(records))
	}
}

func TestUS005_GetReferralInfoReturnsCodeAndStats(t *testing.T) {
	inviter := &model.User{
		Username: "us005-inviter-" + nextAffCode("U"),
		Email:    "us005-inviter-" + nextAffCode("E") + "@example.com",
		AffCode:  "US005-CODE",
		Status:   1,
	}
	if err := testDB.Create(inviter).Error; err != nil {
		t.Fatalf("create inviter failed: %v", err)
	}

	inviteeA := &model.User{Username: "us005-invitee-a-" + nextAffCode("U"), Email: "us005-invitee-a-" + nextAffCode("E") + "@example.com", AffCode: nextAffCode("US005A"), Status: 1}
	inviteeB := &model.User{Username: "us005-invitee-b-" + nextAffCode("U"), Email: "us005-invitee-b-" + nextAffCode("E") + "@example.com", AffCode: nextAffCode("US005B"), Status: 1}
	if err := testDB.Create(inviteeA).Error; err != nil {
		t.Fatalf("create inviteeA failed: %v", err)
	}
	if err := testDB.Create(inviteeB).Error; err != nil {
		t.Fatalf("create inviteeB failed: %v", err)
	}

	rGranted := ttmodel.Referral{
		InviterId:    uint(inviter.Id),
		InviteeId:    uint(inviteeA.Id),
		InviteCode:   inviter.AffCode,
		Status:       "granted",
		BonusUSD:     ttmodel.DefaultReferralConfig.BonusUSD,
		InviterBonus: decimal.NewFromFloat(3.50),
		InviteeBonus: ttmodel.DefaultReferralConfig.BonusUSD,
	}
	rPending := ttmodel.Referral{
		InviterId:    uint(inviter.Id),
		InviteeId:    uint(inviteeB.Id),
		InviteCode:   inviter.AffCode,
		Status:       "pending",
		BonusUSD:     ttmodel.DefaultReferralConfig.BonusUSD,
		InviterBonus: decimal.NewFromFloat(3.50),
		InviteeBonus: ttmodel.DefaultReferralConfig.BonusUSD,
	}
	if err := testDB.Create(&rGranted).Error; err != nil {
		t.Fatalf("create granted referral failed: %v", err)
	}
	if err := testDB.Create(&rPending).Error; err != nil {
		t.Fatalf("create pending referral failed: %v", err)
	}

	info, err := ttmodel.GetReferralInfo(int(inviter.Id))
	if err != nil {
		t.Fatalf("GetReferralInfo failed: %v", err)
	}
	if info.InviteCode != inviter.AffCode {
		t.Fatalf("expected invite_code %s, got %s", inviter.AffCode, info.InviteCode)
	}
	if info.TotalInvites != 2 {
		t.Fatalf("expected total_invites=2, got %d", info.TotalInvites)
	}
	if info.SuccessfulInvites != 1 {
		t.Fatalf("expected successful_invites=1, got %d", info.SuccessfulInvites)
	}
	if info.TotalReward != "3.50" {
		t.Fatalf("expected total_reward=3.50 (granted only), got %s", info.TotalReward)
	}
	if info.AvailableReward != "0.00" {
		t.Fatalf("expected available_reward=0.00 by current behavior, got %s", info.AvailableReward)
	}
}

func TestUS005_GetReferralInfoRejectsUnknownUser(t *testing.T) {
	_, err := ttmodel.GetReferralInfo(999999)
	if err == nil {
		t.Fatalf("expected unknown user query to fail")
	}
}
