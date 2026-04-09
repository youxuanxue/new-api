package tests

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	ttmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

func TestUS060_CreateTeam(t *testing.T) {
	user := &model.User{
		Username: "teamowner",
		Email:    "owner@example.com",
		AffCode:  nextAffCode("US060"),
		Status:   1,
	}
	testDB.Create(user)

	team, err := ttmodel.CreateTeam(uint(user.Id), "Test Team", "Test Description", 0)
	if err != nil {
		t.Errorf("Failed to create team: %v", err)
	}

	if team.OwnerId != uint(user.Id) {
		t.Errorf("Expected owner ID %d, got %d", user.Id, team.OwnerId)
	}

	t.Logf("✓ US-060: Create team test passed")
}

func TestUS060_CreateTeam_OwnerAlreadyMemberAfterCreation(t *testing.T) {
	user := &model.User{
		Username: "teamowner060",
		Email:    "owner060@example.com",
		AffCode:  nextAffCode("US060D"),
		Status:   1,
	}
	testDB.Create(user)

	team, err := ttmodel.CreateTeam(uint(user.Id), "US060 Team", "", 0)
	if err != nil {
		t.Fatalf("Failed to create team: %v", err)
	}

	err = ttmodel.AddTeamMember(team.Id, uint(user.Id), ttmodel.TeamRoleOwner)
	if err == nil {
		t.Fatal("Expected duplicate owner add to fail")
	}
	if !strings.Contains(err.Error(), "already in team") {
		t.Fatalf("Expected duplicate error, got: %v", err)
	}
}

func TestUS061_AddTeamMember(t *testing.T) {
	owner := &model.User{Username: "owner", Email: "owner@team.com", AffCode: nextAffCode("US061O"), Status: 1}
	member := &model.User{Username: "member", Email: "member@team.com", AffCode: nextAffCode("US061M"), Status: 1}
	testDB.Create(owner)
	testDB.Create(member)

	team, _ := ttmodel.CreateTeam(uint(owner.Id), "Team", "", 0)

	err := ttmodel.AddTeamMember(team.Id, uint(member.Id), ttmodel.TeamRoleMember)
	if err != nil {
		t.Errorf("Failed to add team member: %v", err)
	}

	isMember, role := ttmodel.IsTeamMember(team.Id, uint(member.Id))
	if !isMember || role != ttmodel.TeamRoleMember {
		t.Error("Member was not added correctly")
	}

	t.Logf("✓ US-061: Add team member test passed")
}

func TestUS061_AddTeamMember_DuplicateRejected(t *testing.T) {
	owner := &model.User{Username: "owner061dup", Email: "owner061dup@team.com", AffCode: nextAffCode("US061DO"), Status: 1}
	member := &model.User{Username: "member061dup", Email: "member061dup@team.com", AffCode: nextAffCode("US061DM"), Status: 1}
	testDB.Create(owner)
	testDB.Create(member)

	team, _ := ttmodel.CreateTeam(uint(owner.Id), "Team061Dup", "", 0)
	if err := ttmodel.AddTeamMember(team.Id, uint(member.Id), ttmodel.TeamRoleMember); err != nil {
		t.Fatalf("Initial member add failed: %v", err)
	}

	err := ttmodel.AddTeamMember(team.Id, uint(member.Id), ttmodel.TeamRoleMember)
	if err == nil {
		t.Fatal("Expected duplicate member add to fail")
	}
	if !strings.Contains(err.Error(), "already in team") {
		t.Fatalf("Expected duplicate error, got: %v", err)
	}
}

func TestUS062_RemoveTeamMember(t *testing.T) {
	owner := &model.User{Username: "owner062", Email: "owner062@team.com", AffCode: nextAffCode("US062O"), Status: 1}
	member := &model.User{Username: "member062", Email: "member062@team.com", AffCode: nextAffCode("US062M"), Status: 1}
	testDB.Create(owner)
	testDB.Create(member)

	team, _ := ttmodel.CreateTeam(uint(owner.Id), "Team062", "", 0)
	if err := ttmodel.AddTeamMember(team.Id, uint(member.Id), ttmodel.TeamRoleMember); err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	if err := ttmodel.RemoveTeamMember(team.Id, uint(member.Id)); err != nil {
		t.Fatalf("Failed to remove team member: %v", err)
	}

	isMember, _ := ttmodel.IsTeamMember(team.Id, uint(member.Id))
	if isMember {
		t.Fatal("Expected member to be removed from team")
	}
}

func TestUS062_RemoveTeamMember_CannotRemoveOwner(t *testing.T) {
	owner := &model.User{Username: "owner062neg", Email: "owner062neg@team.com", AffCode: nextAffCode("US062NO"), Status: 1}
	testDB.Create(owner)

	team, _ := ttmodel.CreateTeam(uint(owner.Id), "Team062Neg", "", 0)
	err := ttmodel.RemoveTeamMember(team.Id, uint(owner.Id))
	if err == nil {
		t.Fatal("Expected removing owner to fail")
	}
	if !strings.Contains(err.Error(), "cannot remove team owner") {
		t.Fatalf("Expected owner-protection error, got: %v", err)
	}
}

func TestUS063_UpdateMemberRole(t *testing.T) {
	owner := &model.User{Username: "owner063", Email: "owner063@team.com", AffCode: nextAffCode("US063O"), Status: 1}
	member := &model.User{Username: "member063", Email: "member063@team.com", AffCode: nextAffCode("US063M"), Status: 1}
	testDB.Create(owner)
	testDB.Create(member)

	team, _ := ttmodel.CreateTeam(uint(owner.Id), "Team063", "", 0)
	if err := ttmodel.AddTeamMember(team.Id, uint(member.Id), ttmodel.TeamRoleMember); err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	if err := ttmodel.UpdateMemberRole(team.Id, uint(member.Id), ttmodel.TeamRoleAdmin); err != nil {
		t.Fatalf("Failed to update member role: %v", err)
	}

	isMember, role := ttmodel.IsTeamMember(team.Id, uint(member.Id))
	if !isMember || role != ttmodel.TeamRoleAdmin {
		t.Fatalf("Expected member role=%s, got role=%s isMember=%v", ttmodel.TeamRoleAdmin, role, isMember)
	}
}

func TestUS063_UpdateMemberRole_NonMemberNoImplicitAdd(t *testing.T) {
	owner := &model.User{Username: "owner063neg", Email: "owner063neg@team.com", AffCode: nextAffCode("US063NO"), Status: 1}
	outsider := &model.User{Username: "outsider063", Email: "outsider063@team.com", AffCode: nextAffCode("US063NM"), Status: 1}
	testDB.Create(owner)
	testDB.Create(outsider)

	team, _ := ttmodel.CreateTeam(uint(owner.Id), "Team063Neg", "", 0)
	if err := ttmodel.UpdateMemberRole(team.Id, uint(outsider.Id), ttmodel.TeamRoleAdmin); err != nil {
		t.Fatalf("Unexpected update error for non-member: %v", err)
	}

	isMember, _ := ttmodel.IsTeamMember(team.Id, uint(outsider.Id))
	if isMember {
		t.Fatal("Role update should not implicitly add non-member into team")
	}
}

func TestUS064_CreateTeamAPIKey(t *testing.T) {
	owner := &model.User{Username: "owner064", Email: "owner064@team.com", AffCode: nextAffCode("US064O"), Status: 1}
	testDB.Create(owner)
	team, _ := ttmodel.CreateTeam(uint(owner.Id), "Team064", "", 0)

	key, err := ttmodel.CreateTeamAPIKey(team.Id, "key064", "test key")
	if err != nil {
		t.Fatalf("Failed to create team api key: %v", err)
	}
	if key.TeamId != team.Id {
		t.Fatalf("Expected key team_id=%d, got %d", team.Id, key.TeamId)
	}
	if !strings.HasPrefix(key.Key, "tk-team-") {
		t.Fatalf("Expected key prefix tk-team-, got %s", key.Key)
	}
}

func TestUS064_CreateTeamAPIKey_RevokedKeyRateLimitIsZero(t *testing.T) {
	owner := &model.User{Username: "owner064neg", Email: "owner064neg@team.com", AffCode: nextAffCode("US064NO"), Status: 1}
	testDB.Create(owner)
	team, _ := ttmodel.CreateTeam(uint(owner.Id), "Team064Neg", "", 0)

	key, err := ttmodel.CreateTeamAPIKey(team.Id, "key064neg", "test key")
	if err != nil {
		t.Fatalf("Failed to create team api key: %v", err)
	}
	if err := ttmodel.RevokeTeamAPIKey(team.Id, key.Id); err != nil {
		t.Fatalf("Failed to revoke team api key: %v", err)
	}

	rateLimit := ttmodel.GetTeamAPIKeyRateLimit(key.Key)
	if rateLimit != 0 {
		t.Fatalf("Expected revoked key rate limit to be 0, got %d", rateLimit)
	}
}

func TestUS065_ListTeams(t *testing.T) {
	user := &model.User{Username: "user065", Email: "user065@team.com", AffCode: nextAffCode("US065U"), Status: 1}
	ownerA := &model.User{Username: "owner065a", Email: "owner065a@team.com", AffCode: nextAffCode("US065OA"), Status: 1}
	ownerB := &model.User{Username: "owner065b", Email: "owner065b@team.com", AffCode: nextAffCode("US065OB"), Status: 1}
	testDB.Create(user)
	testDB.Create(ownerA)
	testDB.Create(ownerB)

	teamA, _ := ttmodel.CreateTeam(uint(ownerA.Id), "Team065A", "", 0)
	teamB, _ := ttmodel.CreateTeam(uint(ownerB.Id), "Team065B", "", 0)
	if err := ttmodel.AddTeamMember(teamA.Id, uint(user.Id), ttmodel.TeamRoleMember); err != nil {
		t.Fatalf("Failed to add user to team A: %v", err)
	}
	if err := ttmodel.AddTeamMember(teamB.Id, uint(user.Id), ttmodel.TeamRoleAdmin); err != nil {
		t.Fatalf("Failed to add user to team B: %v", err)
	}

	teams, err := ttmodel.GetUserTeams(uint(user.Id))
	if err != nil {
		t.Fatalf("Failed to list user teams: %v", err)
	}
	if len(teams) < 2 {
		t.Fatalf("Expected at least 2 teams, got %d", len(teams))
	}
}

func TestUS065_ListTeams_EmptyForNonMember(t *testing.T) {
	user := &model.User{Username: "user065empty", Email: "user065empty@team.com", AffCode: nextAffCode("US065N"), Status: 1}
	testDB.Create(user)

	teams, err := ttmodel.GetUserTeams(uint(user.Id))
	if err != nil {
		t.Fatalf("Failed to list user teams: %v", err)
	}
	if len(teams) != 0 {
		t.Fatalf("Expected no teams for non-member user, got %d", len(teams))
	}
}

// waitWebhookMinSendCount polls until webhook id reaches at least wantCount (covers async TriggerWebhook).
func waitWebhookMinSendCount(t *testing.T, id uint, wantCount int64, maxWait time.Duration) {
	t.Helper()
	deadline := time.Now().Add(maxWait)
	var w ttmodel.Webhook
	for time.Now().Before(deadline) {
		if err := testDB.First(&w, id).Error; err != nil {
			t.Fatalf("reload webhook %d: %v", id, err)
		}
		if w.SendCount >= wantCount {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("webhook %d: send_count want >= %d, got %d after %v", id, wantCount, w.SendCount, maxWait)
}

func TestUS080_CreateWebhookByAdmin_GeneratesSecret(t *testing.T) {
	raw, err := ttmodel.CreateWebhookByAdmin("US080 Hook", "https://example.com/webhook", "balance_low,budget_alert")
	if err != nil {
		t.Fatalf("CreateWebhookByAdmin: %v", err)
	}
	wh, ok := raw.(ttmodel.Webhook)
	if !ok {
		t.Fatalf("expected Webhook, got %T", raw)
	}
	if len(wh.Secret) != 32 {
		t.Fatalf("expected 32-char signing secret, got len=%d", len(wh.Secret))
	}
	if !wh.IsActive {
		t.Fatal("expected IsActive true for new webhook")
	}

	raw2, err := ttmodel.CreateWebhookByAdmin("US080 B", "https://example.com/webhook-b", "e")
	if err != nil {
		t.Fatalf("second CreateWebhookByAdmin: %v", err)
	}
	wh2 := raw2.(ttmodel.Webhook)
	if wh.Secret == wh2.Secret {
		t.Fatal("expected distinct secrets across two creates (no fixed default)")
	}
}

func TestUS081_SendWebhookRequest_UpdatesLastSentAndCount(t *testing.T) {
	raw, err := ttmodel.CreateWebhookByAdmin("US081", "https://example.com/w81", "budget_alert")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	wh := raw.(ttmodel.Webhook)
	before := wh.SendCount
	if err := ttmodel.SendWebhookRequest(&wh, "budget_alert", map[string]interface{}{"note": "probe"}); err != nil {
		t.Fatalf("SendWebhookRequest: %v", err)
	}
	var got ttmodel.Webhook
	if err := testDB.First(&got, wh.Id).Error; err != nil {
		t.Fatalf("reload webhook: %v", err)
	}
	if got.SendCount != before+1 {
		t.Fatalf("send_count: want %d, got %d", before+1, got.SendCount)
	}
	if got.LastSent == nil {
		t.Fatal("expected LastSent to be set after send")
	}
}

func TestUS081_TriggerWebhook_OnlyMatchingSubscriptions(t *testing.T) {
	a, err := ttmodel.CreateWebhookByAdmin("US081A", "https://a.example/h", "budget_alert")
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := ttmodel.CreateWebhookByAdmin("US081B", "https://b.example/h", "other_only")
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	whA := a.(ttmodel.Webhook)
	whB := b.(ttmodel.Webhook)
	beforeA := whA.SendCount
	beforeB := whB.SendCount

	ttmodel.TriggerWebhook("budget_alert", map[string]interface{}{"level": "warn"})

	waitWebhookMinSendCount(t, whA.Id, beforeA+1, 3*time.Second)

	if err := testDB.First(&whB, whB.Id).Error; err != nil {
		t.Fatalf("reload B: %v", err)
	}
	if whB.SendCount != beforeB {
		t.Fatalf("unsubscribed webhook B must not increment: want %d, got %d", beforeB, whB.SendCount)
	}
	if err := testDB.First(&whA, whA.Id).Error; err != nil {
		t.Fatalf("reload A: %v", err)
	}
	if whA.SendCount != beforeA+1 {
		t.Fatalf("subscribed webhook A: want send_count %d, got %d", beforeA+1, whA.SendCount)
	}
}

func TestUS082_TestWebhookByAdmin_NotFound(t *testing.T) {
	_, err := ttmodel.TestWebhookByAdmin(999_999_001)
	if err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestUS082_TestWebhookByAdmin_Success(t *testing.T) {
	raw, err := ttmodel.CreateWebhookByAdmin("US082", "https://example.com/w82", "test")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	wh := raw.(ttmodel.Webhook)
	before := wh.SendCount
	res, err := ttmodel.TestWebhookByAdmin(wh.Id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}
	success, _ := m["success"].(bool)
	if !success {
		t.Fatalf("expected test success, got %#v", m)
	}
	var after ttmodel.Webhook
	if err := testDB.First(&after, wh.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.SendCount != before+1 {
		t.Fatalf("test path must bump send_count: want %d, got %d", before+1, after.SendCount)
	}
}

func TestUS083_UpdateWebhookByAdmin_ChangesFields(t *testing.T) {
	raw, err := ttmodel.CreateWebhookByAdmin("oldname", "https://u.example/1", "e1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	wh := raw.(ttmodel.Webhook)
	if err := ttmodel.UpdateWebhookByAdmin(wh.Id, "newname", "https://u.example/2", "e2,e3"); err != nil {
		t.Fatalf("update: %v", err)
	}
	var got ttmodel.Webhook
	if err := testDB.First(&got, wh.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Name != "newname" || got.URL != "https://u.example/2" || got.Events != "e2,e3" {
		t.Fatalf("unexpected row: %+v", got)
	}
}

func TestUS083_UpdateWebhookByAdmin_EmptyPayloadNoOp(t *testing.T) {
	raw, err := ttmodel.CreateWebhookByAdmin("keep", "https://u.example/k", "only")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	wh := raw.(ttmodel.Webhook)
	if err := ttmodel.UpdateWebhookByAdmin(wh.Id, "", "", ""); err != nil {
		t.Fatalf("empty update: %v", err)
	}
	var got ttmodel.Webhook
	if err := testDB.First(&got, wh.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Name != "keep" || got.URL != "https://u.example/k" || got.Events != "only" {
		t.Fatalf("empty update must not clear fields, got %+v", got)
	}
}

func TestUS083_DeleteWebhookByAdmin_RemovesRow(t *testing.T) {
	raw, err := ttmodel.CreateWebhookByAdmin("todel", "https://u.example/d", "ev")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	wh := raw.(ttmodel.Webhook)
	if err := ttmodel.DeleteWebhookByAdmin(wh.Id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	err = testDB.First(&ttmodel.Webhook{}, wh.Id).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}
