package tests

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	ttmodel "github.com/QuantumNous/new-api/model"
	ttcontroller "github.com/QuantumNous/new-api/tt/controller"
	"github.com/gin-gonic/gin"
)

func TestUS127_AdminAdjustTeamBalance_UpdatesBalance(t *testing.T) {
	owner := &model.User{Username: "us127own", Email: "us127own@example.com", AffCode: nextAffCode("US127O"), Status: 1}
	testDB.Create(owner)
	team, err := ttmodel.CreateTeam(uint(owner.Id), "US127 Team", "", 0)
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(team.Id))}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/teams/"+strconv.Itoa(int(team.Id))+"/adjust-balance", strings.NewReader(`{"amount":3.5}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("admin_id", 1)
	ttcontroller.AdminAdjustTeamBalance(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("AdminAdjustTeamBalance HTTP %d body=%s", rec.Code, rec.Body.String())
	}

	var got ttmodel.Team
	if err := testDB.First(&got, team.Id).Error; err != nil {
		t.Fatalf("reload team: %v", err)
	}
	if got.Balance < 3.499 || got.Balance > 3.501 {
		t.Fatalf("team balance want ~3.5 got %v", got.Balance)
	}
}

func TestUS127_AdminSetTeamMonthlyLimit_Persists(t *testing.T) {
	owner := &model.User{Username: "us127lim", Email: "us127lim@example.com", AffCode: nextAffCode("US127L"), Status: 1}
	testDB.Create(owner)
	team, err := ttmodel.CreateTeam(uint(owner.Id), "US127 Cap Team", "", 0)
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(team.Id))}}
	ctx.Request = httptest.NewRequest(http.MethodPut, "/admin/teams/"+strconv.Itoa(int(team.Id))+"/monthly-limit", strings.NewReader(`{"monthly_limit":250.5}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("admin_id", 1)
	ttcontroller.AdminSetTeamMonthlyLimit(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("AdminSetTeamMonthlyLimit HTTP %d body=%s", rec.Code, rec.Body.String())
	}

	var got ttmodel.Team
	if err := testDB.First(&got, team.Id).Error; err != nil {
		t.Fatalf("reload team: %v", err)
	}
	if got.MonthlyLimit < 250.499 || got.MonthlyLimit > 250.501 {
		t.Fatalf("monthly_limit want ~250.5 got %v", got.MonthlyLimit)
	}
}
