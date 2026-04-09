package tests

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	ttmodel "github.com/QuantumNous/new-api/model"
	ttcontroller "github.com/QuantumNous/new-api/tt/controller"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestUS070_ListPlans(t *testing.T) {
	plans := []ttmodel.Plan{
		{Name: "Starter", MonthlyPrice: decimal.NewFromFloat(15), IncludedUSD: decimal.NewFromFloat(18), IsActive: true},
		{Name: "Developer", MonthlyPrice: decimal.NewFromFloat(59), IncludedUSD: decimal.NewFromFloat(80), IsActive: true},
		{Name: "Retired070", MonthlyPrice: decimal.NewFromFloat(9), IncludedUSD: decimal.NewFromFloat(5), IsActive: false},
	}
	for _, p := range plans {
		if err := testDB.Create(&p).Error; err != nil {
			t.Fatalf("seed plan %q: %v", p.Name, err)
		}
	}
	if err := testDB.Model(&ttmodel.Plan{}).Where("name = ?", "Retired070").Update("is_active", false).Error; err != nil {
		t.Fatalf("set Retired070 inactive: %v", err)
	}

	result, err := ttmodel.GetActivePlans()
	if err != nil {
		t.Fatalf("Failed to list plans: %v", err)
	}
	if len(result) < 2 {
		t.Fatalf("Expected at least 2 active plans, got %d", len(result))
	}
	names := make(map[string]struct{}, len(result))
	for _, p := range result {
		names[p.Name] = struct{}{}
	}
	for _, want := range []string{"Starter", "Developer"} {
		if _, ok := names[want]; !ok {
			t.Errorf("missing active plan %q in GetActivePlans result", want)
		}
	}
	if _, bad := names["Retired070"]; bad {
		t.Error("inactive plan Retired070 must not appear in GetActivePlans")
	}
	t.Logf("✓ US-070: List plans test passed")
}

func TestUS070_GetActivePlans_IncludesUpstreamSubscriptionPlanId(t *testing.T) {
	p := ttmodel.Plan{
		Name:                       "US070-Upstream",
		MonthlyPrice:               decimal.NewFromFloat(9),
		IncludedUSD:                decimal.NewFromFloat(10),
		IsActive:                   true,
		UpstreamSubscriptionPlanId: 4242,
	}
	if err := testDB.Create(&p).Error; err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	plans, err := ttmodel.GetActivePlans()
	if err != nil {
		t.Fatalf("GetActivePlans: %v", err)
	}
	var got *ttmodel.Plan
	for i := range plans {
		if plans[i].Id == p.Id {
			got = &plans[i]
			break
		}
	}
	if got == nil {
		t.Fatal("seeded plan not found in GetActivePlans")
	}
	if got.UpstreamSubscriptionPlanId != 4242 {
		t.Fatalf("UpstreamSubscriptionPlanId want 4242 got %d", got.UpstreamSubscriptionPlanId)
	}
}

func TestUS070_HTTP_ListPlans_IncludesUpstreamSubscriptionPlanId(t *testing.T) {
	p := ttmodel.Plan{
		Name:                       "US070-HTTP-Up",
		MonthlyPrice:               decimal.NewFromFloat(11),
		IncludedUSD:                decimal.NewFromFloat(12),
		IsActive:                   true,
		UpstreamSubscriptionPlanId: 9001,
	}
	if err := testDB.Create(&p).Error; err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/tt/subscription/plans", nil)
	ttcontroller.ListPlans(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("ListPlans HTTP %d body=%s", rec.Code, rec.Body.String())
	}
	var wrap struct {
		Data []struct {
			Id                         uint `json:"id"`
			UpstreamSubscriptionPlanId int  `json:"upstream_subscription_plan_id"`
		} `json:"data"`
	}
	if err := common.Unmarshal(rec.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	found := false
	for _, row := range wrap.Data {
		if row.Id == p.Id && row.UpstreamSubscriptionPlanId == 9001 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected plan id=%d upstream=9001 in response, got %+v", p.Id, wrap.Data)
	}
}

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

func TestUS071_HTTP_Subscribe_RejectsPlanWithoutUpstreamMapping(t *testing.T) {
	user := &model.User{Username: "sub071http", Email: "sub071http@example.com", AffCode: nextAffCode("US071H"), Status: 1}
	testDB.Create(user)
	plan := ttmodel.Plan{
		Name:                       "Plan071NoUpstream",
		MonthlyPrice:               decimal.NewFromFloat(10),
		IncludedUSD:                decimal.NewFromFloat(20),
		IsActive:                   true,
		UpstreamSubscriptionPlanId: 0,
	}
	testDB.Create(&plan)

	body := `{"plan_id":` + strconv.FormatUint(uint64(plan.Id), 10) + `}`
	ctx, rec := newTTContext(t, http.MethodPost, "/tt/subscription/subscribe", []byte(body), int(user.Id))
	ttcontroller.Subscribe(ctx)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := common.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error.Type != "subscription_error" {
		t.Fatalf("error.type want subscription_error got %q", env.Error.Type)
	}
	if !strings.Contains(env.Error.Message, "upstream_subscription_plan_id") {
		t.Fatalf("expected upstream mapping hint in message, got %q", env.Error.Message)
	}
}

func TestUS072_HTTP_CancelSubscription_InvalidatesUpstreamRow(t *testing.T) {
	user := &model.User{Username: "sub072http", Email: "sub072http@example.com", AffCode: nextAffCode("US072H"), Status: 1}
	testDB.Create(user)

	sp := &ttmodel.SubscriptionPlan{
		Title:         "HTTP Cancel Plan",
		DurationUnit:  ttmodel.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
	}
	if err := testDB.Create(sp).Error; err != nil {
		t.Fatalf("seed subscription plan: %v", err)
	}
	now := time.Now().Unix()
	usub := &ttmodel.UserSubscription{
		UserId:      int(user.Id),
		PlanId:      sp.Id,
		Status:      "active",
		StartTime:   now,
		EndTime:     now + 86400*400,
		AmountTotal: 1_000_000,
		AmountUsed:  0,
	}
	if err := testDB.Create(usub).Error; err != nil {
		t.Fatalf("seed user subscription: %v", err)
	}

	form := "reason=" + url.QueryEscape("integration-test")
	req := httptest.NewRequest(http.MethodPost, "/tt/subscription/cancel", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = req
	ctx.Set("id", int(user.Id))
	ttcontroller.CancelSubscription(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("CancelSubscription HTTP %d body=%s", rec.Code, rec.Body.String())
	}

	after := time.Now().Unix()
	var reloaded ttmodel.UserSubscription
	if err := testDB.First(&reloaded, usub.Id).Error; err != nil {
		t.Fatalf("reload subscription: %v", err)
	}
	if reloaded.Status != "cancelled" {
		t.Fatalf("upstream subscription status want cancelled got %q", reloaded.Status)
	}
	if reloaded.EndTime > after+120 {
		t.Fatalf("expected end_time near cancellation (<= after+120), got end=%d after=%d", reloaded.EndTime, after)
	}
}

func TestUS073_HTTP_GetSubscription_PrefersUpstreamSubscription(t *testing.T) {
	user := &model.User{Username: "sub073http", Email: "sub073http@example.com", AffCode: nextAffCode("US073H"), Status: 1}
	testDB.Create(user)

	sp := &ttmodel.SubscriptionPlan{
		Title:         "HTTP View Upstream Title",
		DurationUnit:  ttmodel.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
	}
	if err := testDB.Create(sp).Error; err != nil {
		t.Fatalf("seed subscription plan: %v", err)
	}
	now := time.Now().Unix()
	usub := &ttmodel.UserSubscription{
		UserId:      int(user.Id),
		PlanId:      sp.Id,
		Status:      "active",
		StartTime:   now,
		EndTime:     now + 86400*400,
		AmountTotal: 1_000_000,
		AmountUsed:  0,
	}
	if err := testDB.Create(usub).Error; err != nil {
		t.Fatalf("seed user subscription: %v", err)
	}

	ctx, rec := newTTContext(t, http.MethodGet, "/tt/subscription", nil, int(user.Id))
	ttcontroller.GetSubscription(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("GetSubscription HTTP %d body=%s", rec.Code, rec.Body.String())
	}
	var info ttcontroller.SubscriptionInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !info.HasSubscription || info.Status != "active" {
		t.Fatalf("unexpected subscription payload: %+v", info)
	}
	if info.PlanName != sp.Title {
		t.Fatalf("PlanName want %q got %q", sp.Title, info.PlanName)
	}
	if info.ExpiresAt == "" {
		t.Fatal("expected ExpiresAt from upstream end_time")
	}
}
