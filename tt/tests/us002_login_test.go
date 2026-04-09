package tests

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func mustCreateLoginUser(t *testing.T, username string, email string, password string, status int) *model.User {
	t.Helper()

	hashed, err := common.Password2Hash(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := &model.User{
		Username: username,
		Email:    email,
		Password: hashed,
		AffCode:  nextAffCode("US002"),
		Status:   status,
	}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatalf("failed to create login user: %v", err)
	}
	return user
}

// TestUS002_LoginWithUsernameSuccess 正向：用户名+正确密码可通过认证
func TestUS002_LoginWithUsernameSuccess(t *testing.T) {
	username := "login-user-" + nextAffCode("U")
	password := "Passw0rd!US002"
	created := mustCreateLoginUser(t, username, username+"@example.com", password, common.UserStatusEnabled)

	loginUser := &model.User{
		Username: username,
		Password: password,
	}
	if err := loginUser.ValidateAndFill(); err != nil {
		t.Fatalf("expected successful login, got error: %v", err)
	}
	if loginUser.Id != created.Id {
		t.Errorf("expected resolved user id %d, got %d", created.Id, loginUser.Id)
	}
	if loginUser.Status != common.UserStatusEnabled {
		t.Errorf("expected enabled status %d, got %d", common.UserStatusEnabled, loginUser.Status)
	}

	t.Logf("✓ US-002: username login success test passed")
}

// TestUS002_LoginWithEmailSuccess 正向：邮箱+正确密码可通过认证
func TestUS002_LoginWithEmailSuccess(t *testing.T) {
	username := "login-email-" + nextAffCode("U")
	email := username + "@example.com"
	password := "Passw0rd!US002"
	created := mustCreateLoginUser(t, username, email, password, common.UserStatusEnabled)

	loginUser := &model.User{
		Username: email, // ValidateAndFill 支持 username/email 二选一输入
		Password: password,
	}
	if err := loginUser.ValidateAndFill(); err != nil {
		t.Fatalf("expected successful email login, got error: %v", err)
	}
	if loginUser.Id != created.Id {
		t.Errorf("expected resolved user id %d, got %d", created.Id, loginUser.Id)
	}

	t.Logf("✓ US-002: email login success test passed")
}

// TestUS002_LoginWrongPasswordRejected 负向：错误密码必须拒绝
func TestUS002_LoginWrongPasswordRejected(t *testing.T) {
	username := "login-wrong-pass-" + nextAffCode("U")
	password := "Passw0rd!US002"
	mustCreateLoginUser(t, username, username+"@example.com", password, common.UserStatusEnabled)

	loginUser := &model.User{
		Username: username,
		Password: "WrongPassword!US002",
	}
	if err := loginUser.ValidateAndFill(); err == nil {
		t.Fatal("expected login failure for wrong password, got success")
	}

	t.Logf("✓ US-002: wrong password rejection test passed")
}

// TestUS002_LoginDisabledUserRejected 负向：禁用用户必须拒绝
func TestUS002_LoginDisabledUserRejected(t *testing.T) {
	username := "login-disabled-" + nextAffCode("U")
	password := "Passw0rd!US002"
	mustCreateLoginUser(t, username, username+"@example.com", password, common.UserStatusDisabled)

	loginUser := &model.User{
		Username: username,
		Password: password,
	}
	if err := loginUser.ValidateAndFill(); err == nil {
		t.Fatal("expected login failure for disabled user, got success")
	}

	t.Logf("✓ US-002: disabled user rejection test passed")
}
