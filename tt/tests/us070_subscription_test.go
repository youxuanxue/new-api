package tests

import (
	"errors"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestUS071_SubscribePlan(t *testing.T) {
	user := &model.User{Username: "sub071", Email: "sub071@example.com", AffCode: nextAffCode("US071"), Status: 1}
	testDB.Create(user)
	plan := ttmodel.Plan{
		Name:         "Plan071",
		MonthlyPrice: decimal.NewFromFloat(10),
		IncludedUSD:  decimal.NewFromFloat(20),
		IsActive:     true,
	}
	testDB.Create(&plan)

	sub, err := ttmodel.CreateSubscription(int(user.Id), plan.Id, "monthly")
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}
	if sub.Status != "active" || sub.PlanId != plan.Id {
		t.Fatalf("unexpected subscription: status=%q planId=%d", sub.Status, sub.PlanId)
	}
	if !sub.RemainingUSD.Equal(plan.IncludedUSD) {
		t.Fatalf("RemainingUSD want %s got %s", plan.IncludedUSD.String(), sub.RemainingUSD.String())
	}

	info, err := ttmodel.GetUserSubscription(int(user.Id))
	if err != nil {
		t.Fatalf("GetUserSubscription: %v", err)
	}
	if !info.HasSubscription || info.Status != "active" || info.PlanName != plan.Name {
		t.Fatalf("subscription info: %+v", info)
	}

	t.Logf("✓ US-071: Subscribe plan test passed")
}

func TestUS071_SubscribePlan_DuplicateActiveRejected(t *testing.T) {
	user := &model.User{Username: "sub071d", Email: "sub071d@example.com", AffCode: nextAffCode("US071D"), Status: 1}
	testDB.Create(user)
	plan := ttmodel.Plan{
		Name:         "Plan071Dup",
		MonthlyPrice: decimal.NewFromFloat(10),
		IncludedUSD:  decimal.NewFromFloat(20),
		IsActive:     true,
	}
	testDB.Create(&plan)

	if _, err := ttmodel.CreateSubscription(int(user.Id), plan.Id, "monthly"); err != nil {
		t.Fatalf("first subscription: %v", err)
	}
	_, err := ttmodel.CreateSubscription(int(user.Id), plan.Id, "monthly")
	if err == nil {
		t.Fatal("expected error for second active subscription")
	}
	if !strings.Contains(err.Error(), "已有活跃订阅") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUS071_SubscribePlan_PlanNotFoundRejected(t *testing.T) {
	user := &model.User{Username: "sub071nf", Email: "sub071nf@example.com", AffCode: nextAffCode("US071NF"), Status: 1}
	testDB.Create(user)

	_, err := ttmodel.CreateSubscription(int(user.Id), 999_999, "monthly")
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
	if err.Error() != "套餐不存在" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUS072_CancelSubscription(t *testing.T) {
	user := &model.User{Username: "sub072", Email: "sub072@example.com", AffCode: nextAffCode("US072"), Status: 1}
	testDB.Create(user)
	plan := ttmodel.Plan{
		Name:         "Plan072",
		MonthlyPrice: decimal.NewFromFloat(10),
		IncludedUSD:  decimal.NewFromFloat(20),
		IsActive:     true,
	}
	testDB.Create(&plan)
	if _, err := ttmodel.CreateSubscription(int(user.Id), plan.Id, "monthly"); err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}

	reason := "user requested"
	if err := ttmodel.CancelUserSubscription(int(user.Id), reason); err != nil {
		t.Fatalf("CancelUserSubscription: %v", err)
	}

	var sub ttmodel.Subscription
	if err := testDB.Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).First(&sub).Error; err != nil {
		t.Fatalf("reload subscription: %v", err)
	}
	if sub.Status != "cancelled" {
		t.Fatalf("expected status cancelled, got %q", sub.Status)
	}
	if sub.CancelReason != reason {
		t.Fatalf("cancel_reason want %q got %q", reason, sub.CancelReason)
	}
	if sub.CancelledAt == nil {
		t.Fatal("expected cancelled_at to be set")
	}

	_, err := ttmodel.GetUserSubscription(int(user.Id))
	if err == nil {
		t.Fatal("expected GetUserSubscription to fail after cancel (no active row)")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}

	t.Logf("✓ US-072: Cancel subscription test passed")
}

func TestUS072_CancelSubscription_IdempotentWhenNoActive(t *testing.T) {
	user := &model.User{Username: "sub072idle", Email: "sub072idle@example.com", AffCode: nextAffCode("US072I"), Status: 1}
	testDB.Create(user)

	if err := ttmodel.CancelUserSubscription(int(user.Id), "noop"); err != nil {
		t.Fatalf("CancelUserSubscription with no active sub: %v", err)
	}
	var n int64
	testDB.Model(&ttmodel.Subscription{}).Where("user_id = ? AND status = ?", user.Id, "active").Count(&n)
	if n != 0 {
		t.Fatalf("expected zero active subscriptions, got %d", n)
	}
}

func TestUS072_CancelSubscription_DoesNotAffectOtherUser(t *testing.T) {
	userA := &model.User{Username: "sub072a", Email: "sub072a@example.com", AffCode: nextAffCode("US072A"), Status: 1}
	userB := &model.User{Username: "sub072b", Email: "sub072b@example.com", AffCode: nextAffCode("US072B"), Status: 1}
	testDB.Create(userA)
	testDB.Create(userB)
	plan := ttmodel.Plan{
		Name:         "Plan072Iso",
		MonthlyPrice: decimal.NewFromFloat(10),
		IncludedUSD:  decimal.NewFromFloat(20),
		IsActive:     true,
	}
	testDB.Create(&plan)
	if _, err := ttmodel.CreateSubscription(int(userA.Id), plan.Id, "monthly"); err != nil {
		t.Fatalf("CreateSubscription A: %v", err)
	}

	if err := ttmodel.CancelUserSubscription(int(userB.Id), "wrong user"); err != nil {
		t.Fatalf("CancelUserSubscription B: %v", err)
	}

	info, err := ttmodel.GetUserSubscription(int(userA.Id))
	if err != nil {
		t.Fatalf("user A should still have active subscription: %v", err)
	}
	if info.Status != "active" {
		t.Fatalf("user A subscription was affected, status=%q", info.Status)
	}
}

func TestUS073_ViewSubscription(t *testing.T) {
	user := &model.User{Username: "sub073", Email: "sub073@example.com", AffCode: nextAffCode("US073"), Status: 1}
	testDB.Create(user)
	plan := ttmodel.Plan{
		Name:         "Plan073",
		MonthlyPrice: decimal.NewFromFloat(10),
		IncludedUSD:  decimal.NewFromFloat(25.5),
		IsActive:     true,
	}
	testDB.Create(&plan)
	if _, err := ttmodel.CreateSubscription(int(user.Id), plan.Id, "yearly"); err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}

	info, err := ttmodel.GetUserSubscription(int(user.Id))
	if err != nil {
		t.Fatalf("GetUserSubscription: %v", err)
	}
	if !info.HasSubscription || info.PlanName != plan.Name || info.Status != "active" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.RemainingUSD != "25.50" {
		t.Fatalf("RemainingUSD want 25.50 got %q", info.RemainingUSD)
	}

	t.Logf("✓ US-073: View subscription test passed")
}

func TestUS073_ViewSubscription_NoActiveReturnsNotFound(t *testing.T) {
	user := &model.User{Username: "sub073none", Email: "sub073none@example.com", AffCode: nextAffCode("US073N"), Status: 1}
	testDB.Create(user)

	_, err := ttmodel.GetUserSubscription(int(user.Id))
	if err == nil {
		t.Fatal("expected error when user has no active subscription")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}
