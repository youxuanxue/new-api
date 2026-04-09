//go:build tt
// +build tt

package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// tryTeamAPIKeyAuth authenticates tk-team-... API keys and populates the Gin context like a user token.
func tryTeamAPIKeyAuth(c *gin.Context, authHeader string) (handled bool, err error) {
	raw := strings.TrimSpace(authHeader)
	if raw == "" {
		return false, nil
	}
	if strings.HasPrefix(raw, "Bearer ") || strings.HasPrefix(raw, "bearer ") {
		raw = strings.TrimSpace(raw[7:])
	}
	if raw == "" {
		return false, nil
	}
	raw = strings.TrimPrefix(raw, "sk-")
	if !strings.HasPrefix(raw, "tk-team-") {
		return false, nil
	}

	keyRow, team, err := model.GetTeamAPIKeyForRelay(raw)
	if err != nil {
		return true, fmt.Errorf("无效的团队 API Key")
	}
	if keyRow.ExpiresAt != nil && !keyRow.ExpiresAt.After(time.Now()) {
		return true, fmt.Errorf("团队 API Key 已过期")
	}
	if team.Status != "active" {
		return true, fmt.Errorf("团队已停用")
	}

	userCache, err := model.GetUserCache(int(team.OwnerId))
	if err != nil {
		return true, err
	}
	if userCache.Status != common.UserStatusEnabled {
		return true, fmt.Errorf("用户已被封禁")
	}

	common.SetContextKey(c, constant.ContextKeyTeamId, int(team.Id))
	common.SetContextKey(c, constant.ContextKeyTeamAPIKeyId, int(keyRow.Id))

	c.Set("id", int(team.OwnerId))
	c.Set("token_id", 0)
	c.Set("token_unlimited", true)
	c.Set("token_key", raw)
	c.Set("token_name", keyRow.Name)

	userCache.WriteContext(c)

	userGroup := userCache.Group
	if !ratio_setting.ContainsGroupRatio(userGroup) {
		if userGroup != "auto" {
			return true, fmt.Errorf("分组 %s 已被弃用", userGroup)
		}
	}
	common.SetContextKey(c, constant.ContextKeyUsingGroup, userGroup)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "")

	c.Set("token_model_limit_enabled", false)

	return true, nil
}
