package tests

import (
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/model"
	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var testDB *gorm.DB
var affSeq uint64

func nextAffCode(prefix string) string {
	n := atomic.AddUint64(&affSeq, 1)
	return prefix + "-" + strconv.FormatUint(n, 10)
}

// TestMain 初始化测试环境
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	var err error
	testDB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	testDB.AutoMigrate(
		&model.User{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.Channel{},
		&ttmodel.Token{},
		&ttmodel.UserExtension{},
		&ttmodel.Referral{},
		&ttmodel.Plan{},
		&ttmodel.Subscription{},
		&ttmodel.ConsumptionRecord{},
		&ttmodel.Payment{},
		&ttmodel.ModelPricing{},
		&ttmodel.PoolAccount{},
		&ttmodel.Admin{},
		&ttmodel.AdminAuditLog{},
		&ttmodel.Team{},
		&ttmodel.TeamMember{},
		&ttmodel.TeamAPIKey{},
		&ttmodel.Webhook{},
		&ttmodel.UserBudgetConfig{},
		&ttmodel.SSOConfig{},
		&ttmodel.SLAConfig{},
		&ttmodel.SLAReport{},
		&ttmodel.SLAIncident{},
		&ttmodel.SLABreach{},
	)

	sqlDB, err := testDB.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	// SQLite :memory: is per connection; without this, pooled connections see empty DBs and
	// async model code (e.g. TriggerWebhook goroutines) hits "no such table".
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	model.DB = testDB
	ttmodel.DB = testDB

	m.Run()
}
