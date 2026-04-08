package model

import (
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
)

// BootstrapSetupFromEnv creates root user + setup row + default operation options on first boot
// when ADMIN_PASSWORD is set, mirroring POST /api/setup so the web wizard is skipped.
// Safe to call on every startup: no-ops if a setup record already exists or a root user exists.
func BootstrapSetupFromEnv() {
	if DB == nil {
		return
	}
	if GetSetup() != nil {
		return
	}
	if RootUserExists() {
		return
	}

	password := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	if password == "" {
		return
	}
	if len(password) < 8 {
		common.SysLog("ADMIN_PASSWORD is set but shorter than 8 characters; env bootstrap skipped (use setup wizard)")
		return
	}

	username := strings.TrimSpace(os.Getenv("ADMIN_USERNAME"))
	if username == "" {
		username = "admin"
	}
	if len(username) > 12 {
		common.SysLog("ADMIN_USERNAME exceeds 12 characters; env bootstrap skipped (use setup wizard)")
		return
	}

	selfUse, demo := parseSetupUsageMode(os.Getenv("TT_SETUP_USAGE_MODE"))

	hashedPassword, err := common.Password2Hash(password)
	if err != nil {
		common.SysError("env bootstrap: hash password failed: " + err.Error())
		return
	}

	rootUser := User{
		Username:    username,
		Password:    hashedPassword,
		Role:        common.RoleRootUser,
		Status:      common.UserStatusEnabled,
		DisplayName: "Root User",
		AccessToken: nil,
		Quota:       100000000,
	}

	setupRow := Setup{
		Version:       common.Version,
		InitializedAt: time.Now().Unix(),
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rootUser).Error; err != nil {
			return err
		}
		if err := txFirstOrSaveOption(tx, "SelfUseModeEnabled", boolString(selfUse)); err != nil {
			return err
		}
		if err := txFirstOrSaveOption(tx, "DemoSiteEnabled", boolString(demo)); err != nil {
			return err
		}
		return tx.Create(&setupRow).Error
	})
	if err != nil {
		common.SysError("env bootstrap failed: " + err.Error())
		return
	}

	operation_setting.SelfUseModeEnabled = selfUse
	operation_setting.DemoSiteEnabled = demo
	constant.Setup = true
	common.SysLog("system initialized from ADMIN_USERNAME / ADMIN_PASSWORD (setup wizard skipped)")
}

func parseSetupUsageMode(raw string) (selfUse, demo bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "self", "self_use", "self-use":
		return true, false
	case "demo":
		return false, true
	default:
		// external / empty / unknown → same default as setup wizard "对外运营"
		return false, false
	}
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func txFirstOrSaveOption(tx *gorm.DB, key, value string) error {
	option := Option{Key: key}
	if err := tx.FirstOrCreate(&option, Option{Key: key}).Error; err != nil {
		return err
	}
	option.Value = value
	return tx.Save(&option).Error
}
