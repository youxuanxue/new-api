package tests

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

// TestUS001_UserSignup 正向路径测试
func TestUS001_UserSignup(t *testing.T) {
	user := &model.User{
		Username: "testuser",
		Email:    "test@example.com",
		AffCode:  nextAffCode("US001"),
		Status:   1,
	}
	result := testDB.Create(user)
	if result.Error != nil {
		t.Errorf("Failed to create user: %v", result.Error)
	}
	if user.Status != 1 {
		t.Errorf("Expected user status 1, got %d", user.Status)
	}
	if user.Id == 0 {
		t.Error("Expected persisted user ID, got 0")
	}

	ext, err := ttmodel.GetUserExtension(int(user.Id))
	if err != nil {
		t.Errorf("Failed to get user extension: %v", err)
	}
	if ext.UserId != uint(user.Id) {
		t.Errorf("Expected extension user_id %d, got %d", user.Id, ext.UserId)
	}

	if err := ttmodel.GrantTrialBalance(int(user.Id)); err != nil {
		t.Errorf("Failed to grant trial balance: %v", err)
	}
	ext, _ = ttmodel.GetUserExtension(int(user.Id))

	if ext.TrialBalance.Cmp(decimal.NewFromFloat(1.0)) != 0 {
		t.Errorf("Expected trial balance $1.0, got %s", ext.TrialBalance.String())
	}
	if !ext.TrialUsed.Equal(decimal.Zero) {
		t.Errorf("Expected trial_used 0, got %s", ext.TrialUsed.String())
	}
	if ext.TrialGrantedAt == nil {
		t.Error("Expected trial granted timestamp to be set")
	}

	t.Logf("✓ US-001: User signup test passed")
}

// TestUS001_DuplicateEmail 输入空间测试 - 业务要求邮箱唯一，重复应被拒绝
func TestUS001_DuplicateEmail(t *testing.T) {
	user1 := &model.User{
		Username: "user1",
		Email:    "duplicate@example.com",
		AffCode:  nextAffCode("US001D1"),
		Status:   1,
	}
	if err := testDB.Create(user1).Error; err != nil {
		t.Fatalf("Failed to create first duplicate-email user: %v", err)
	}

	user2 := &model.User{
		Username: "user2",
		Email:    "duplicate@example.com",
		AffCode:  nextAffCode("US001D2"),
		Status:   1,
	}
	result := testDB.Create(user2)
	if result.Error == nil {
		t.Error("Expected duplicate email to be rejected, but got no error")
	} else {
		t.Logf("duplicate email rejected as expected: %v", result.Error)
	}

	t.Logf("✓ US-001: Duplicate email rejection test passed")
}

// TestUS001_TrialGrantIdempotentConsistency 一致性测试：
// 多次发放不应重复累计，也不应创建重复扩展记录。
func TestUS001_TrialGrantIdempotentConsistency(t *testing.T) {
	user := &model.User{
		Username: "trial-idempotent",
		Email:    "trial-idempotent@example.com",
		AffCode:  nextAffCode("US001I"),
		Status:   1,
	}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	if err := ttmodel.GrantTrialBalance(int(user.Id)); err != nil {
		t.Fatalf("First grant failed: %v", err)
	}
	ext1, err := ttmodel.GetUserExtension(int(user.Id))
	if err != nil {
		t.Fatalf("Failed to read extension after first grant: %v", err)
	}
	firstBalance := ext1.TrialBalance

	if err := ttmodel.GrantTrialBalance(int(user.Id)); err != nil {
		t.Fatalf("Second grant failed: %v", err)
	}
	ext2, err := ttmodel.GetUserExtension(int(user.Id))
	if err != nil {
		t.Fatalf("Failed to read extension after second grant: %v", err)
	}
	if !ext2.TrialBalance.Equal(firstBalance) {
		t.Errorf("Expected idempotent grant balance %s, got %s", firstBalance.String(), ext2.TrialBalance.String())
	}

	var extCount int64
	if err := testDB.Model(&ttmodel.UserExtension{}).Where("user_id = ?", user.Id).Count(&extCount).Error; err != nil {
		t.Fatalf("Failed to count user extensions: %v", err)
	}
	if extCount != 1 {
		t.Errorf("Expected exactly 1 user_extension row, got %d", extCount)
	}

	t.Logf("✓ US-001: Trial grant idempotent consistency test passed")
}
