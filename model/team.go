//go:build tt
// +build tt

package model

import (
	"crypto/rand"
	"errors"
	"time"

	"gorm.io/gorm"
)

// QuotaUnitsPerUSD matches TT balance display: user quota / QuotaUnitsPerUSD ≈ USD.
const QuotaUnitsPerUSD = 500000

// ========== 团队 ==========

// Team 团队
type Team struct {
	Id          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:64;not null"`
	Description string    `json:"description" gorm:"size:256"`
	OwnerId     uint      `json:"owner_id" gorm:"not null"`
	PlanId      uint      `json:"plan_id"` // 关联套餐

	// 额度
	Balance      float64 `json:"balance" gorm:"default:0"`
	UsedQuota    int     `json:"used_quota" gorm:"default:0"`
	MonthlyLimit float64 `json:"monthly_limit" gorm:"default:0"` // 月限额

	// 月度用量（配额单位），随自然月重置；与 MonthlyLimit（USD）配合使用
	BillingPeriodYm      int `json:"billing_period_ym" gorm:"default:0"`       // YYYYMM
	MonthlyConsumedUnits int `json:"monthly_consumed_units" gorm:"default:0"` // quota units this month

	// 状态
	Status      string    `json:"status" gorm:"size:20;default:'active'"` // active/suspended
	MemberCount int       `json:"member_count" gorm:"default:1"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// 关联
	Owner    *User        `json:"owner,omitempty" gorm:"foreignKey:OwnerId"`
	Members  []TeamMember `json:"members,omitempty" gorm:"foreignKey:TeamId"`
}

// TableName 指定表名
func (Team) TableName() string {
	return "teams"
}

// TeamMember 团队成员
type TeamMember struct {
	Id       uint   `json:"id" gorm:"primaryKey"`
	TeamId   uint   `json:"team_id" gorm:"not null;index"`
	UserId   uint   `json:"user_id" gorm:"not null;index"`
	Role     string `json:"role" gorm:"size:20;default:'member'"` // owner/admin/member
	Status   string `json:"status" gorm:"size:20;default:'active'"`
	JoinedAt time.Time `json:"joined_at" gorm:"autoCreateTime"`

	// 关联
	User *User `json:"user,omitempty" gorm:"foreignKey:UserId"`
}

// TableName 指定表名
func (TeamMember) TableName() string {
	return "team_members"
}

// 团队角色常量
const (
	TeamRoleOwner  = "owner"
	TeamRoleAdmin  = "admin"
	TeamRoleMember = "member"
)

// ========== 团队管理服务 ==========

// CreateTeam 创建团队
func CreateTeam(ownerId uint, name, description string, planId uint) (*Team, error) {
	team := &Team{
		Name:        name,
		Description: description,
		OwnerId:     ownerId,
		PlanId:      planId,
		Status:      "active",
		MemberCount: 1,
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		// 创建团队
		if err := tx.Create(team).Error; err != nil {
			return err
		}

		// 添加创建者为owner
		member := &TeamMember{
			TeamId: team.Id,
			UserId: ownerId,
			Role:   TeamRoleOwner,
			Status: "active",
		}
		return tx.Create(member).Error
	})

	return team, err
}

// GetTeam 获取团队信息
func GetTeam(teamId uint) (*Team, error) {
	var team Team
	err := DB.Preload("Owner").Preload("Members.User").First(&team, teamId).Error
	if err != nil {
		return nil, err
	}
	return &team, nil
}

// GetUserTeams 获取用户所属团队列表
func GetUserTeams(userId uint) ([]Team, error) {
	var teams []Team
	err := DB.Joins("JOIN team_members ON team_members.team_id = teams.id").
		Where("team_members.user_id = ? AND team_members.status = ?", userId, "active").
		Find(&teams).Error
	return teams, err
}

// AddTeamMember 添加团队成员
func AddTeamMember(teamId, userId uint, role string) error {
	// 检查用户是否已在团队中
	var count int64
	DB.Model(&TeamMember{}).Where("team_id = ? AND user_id = ?", teamId, userId).Count(&count)
	if count > 0 {
		return errors.New("user already in team")
	}

	// 检查团队人数限制
	team, err := GetTeam(teamId)
	if err != nil {
		return err
	}

	var maxMembers int = 5 // 默认值
	if team.PlanId > 0 {
		var plan Plan
		if DB.First(&plan, team.PlanId).Error == nil {
			maxMembers = plan.MaxTeamMembers
		}
	}

	if team.MemberCount >= maxMembers {
		return errors.New("team member limit reached")
	}

	member := &TeamMember{
		TeamId: teamId,
		UserId: userId,
		Role:   role,
		Status: "active",
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(member).Error; err != nil {
			return err
		}
		return tx.Model(&Team{}).Where("id = ?", teamId).
			Update("member_count", gorm.Expr("member_count + 1")).Error
	})
}

// RemoveTeamMember 移除团队成员
func RemoveTeamMember(teamId, userId uint) error {
	// 不能移除owner
	var member TeamMember
	err := DB.Where("team_id = ? AND user_id = ?", teamId, userId).First(&member).Error
	if err != nil {
		return err
	}

	if member.Role == TeamRoleOwner {
		return errors.New("cannot remove team owner")
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&member).Error; err != nil {
			return err
		}
		return tx.Model(&Team{}).Where("id = ?", teamId).
			Update("member_count", gorm.Expr("member_count - 1")).Error
	})
}

// UpdateMemberRole 更新成员角色
func UpdateMemberRole(teamId, userId uint, role string) error {
	return DB.Model(&TeamMember{}).
		Where("team_id = ? AND user_id = ?", teamId, userId).
		Update("role", role).Error
}

// IsTeamMember 检查用户是否是团队成员
func IsTeamMember(teamId, userId uint) (bool, string) {
	var member TeamMember
	err := DB.Where("team_id = ? AND user_id = ? AND status = ?", teamId, userId, "active").First(&member).Error
	if err != nil {
		return false, ""
	}
	return true, member.Role
}

// GetTeamMembers 获取团队成员列表
func GetTeamMembers(teamId uint) ([]TeamMember, error) {
	var members []TeamMember
	err := DB.Preload("User").Where("team_id = ? AND status = ?", teamId, "active").Find(&members).Error
	return members, err
}

// ========== 团队额度管理 ==========

// AdjustTeamBalance 调整团队额度
func AdjustTeamBalance(teamId uint, amount float64) error {
	return DB.Model(&Team{}).Where("id = ?", teamId).
		Update("balance", gorm.Expr("balance + ?", amount)).Error
}

// SetTeamMonthlyLimit 设置团队月限额
func SetTeamMonthlyLimit(teamId uint, limit float64) error {
	return DB.Model(&Team{}).Where("id = ?", teamId).
		Update("monthly_limit", limit).Error
}

// ========== 团队API Key ==========

// TeamAPIKey 团队API Key
type TeamAPIKey struct {
	Id          uint      `json:"id" gorm:"primaryKey"`
	TeamId      uint      `json:"team_id" gorm:"not null;index"`
	Key         string    `json:"key" gorm:"size:64;uniqueIndex;not null"`
	Name        string    `json:"name" gorm:"size:64"`
	Description string    `json:"description" gorm:"size:256"`
	Status      string    `json:"status" gorm:"size:20;default:'active'"`
	RateLimit   int       `json:"rate_limit" gorm:"default:60"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

// TableName 指定表名
func (TeamAPIKey) TableName() string {
	return "team_api_keys"
}

// CreateTeamAPIKey 创建团队API Key
func CreateTeamAPIKey(teamId uint, name, description string) (*TeamAPIKey, error) {
	key := &TeamAPIKey{
		TeamId:      teamId,
		Key:         generateTeamAPIKey(),
		Name:        name,
		Description: description,
		Status:      "active",
		RateLimit:   60,
	}
	err := DB.Create(key).Error
	return key, err
}

// generateTeamAPIKey 生成团队API Key
func generateTeamAPIKey() string {
	// 生成 tk-team- 前缀的API Key
	return "tk-team-" + randomString(32)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		}
	} else {
		for i := range b {
			b[i] = letters[int(b[i])%len(letters)]
		}
	}
	return string(b)
}

// ListTeamAPIKeys 列出团队API Key
func ListTeamAPIKeys(teamId uint) ([]TeamAPIKey, error) {
	var keys []TeamAPIKey
	err := DB.Where("team_id = ?", teamId).Find(&keys).Error
	return keys, err
}

// RevokeTeamAPIKey 撤销团队API Key（必须属于指定团队）
func RevokeTeamAPIKey(teamId, keyId uint) error {
	res := DB.Model(&TeamAPIKey{}).Where("id = ? AND team_id = ? AND status = ?", keyId, teamId, "active").
		Update("status", "revoked")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("api key not found or not in team")
	}
	return nil
}

// GetTeamAPIKeyForRelay 根据完整密钥解析团队与密钥行（仅 active）
func GetTeamAPIKeyForRelay(fullKey string) (*TeamAPIKey, *Team, error) {
	var key TeamAPIKey
	if err := DB.Where("key = ? AND status = ?", fullKey, "active").First(&key).Error; err != nil {
		return nil, nil, err
	}
	var team Team
	if err := DB.First(&team, key.TeamId).Error; err != nil {
		return nil, nil, err
	}
	return &key, &team, nil
}

func currentBillingPeriodYM() int {
	now := time.Now()
	return now.Year()*100 + int(now.Month())
}

// EnsureTeamBillingPeriod resets monthly counters when calendar month changes.
func EnsureTeamBillingPeriod(teamId uint) error {
	ym := currentBillingPeriodYM()
	return DB.Model(&Team{}).Where("id = ? AND billing_period_ym <> ?", teamId, ym).
		Updates(map[string]interface{}{
			"billing_period_ym":        ym,
			"monthly_consumed_units":   0,
		}).Error
}

// QuotaUnitsFromUSD converts USD balance to comparable quota units (floor).
func QuotaUnitsFromUSD(usd float64) int {
	if usd <= 0 {
		return 0
	}
	return int(usd * float64(QuotaUnitsPerUSD))
}

// USDFromQuotaUnits converts quota units to USD.
func USDFromQuotaUnits(units int) float64 {
	if units <= 0 {
		return 0
	}
	return float64(units) / float64(QuotaUnitsPerUSD)
}

// GetTeamSpendableQuotaUnits returns remaining quota units from team USD balance after syncing billing month.
func GetTeamSpendableQuotaUnits(teamId uint) (int, error) {
	if err := EnsureTeamBillingPeriod(teamId); err != nil {
		return 0, err
	}
	var team Team
	if err := DB.First(&team, teamId).Error; err != nil {
		return 0, err
	}
	if team.Status != "active" {
		return 0, errors.New("team is not active")
	}
	units := QuotaUnitsFromUSD(team.Balance)
	if team.MonthlyLimit > 0 {
		capUnits := QuotaUnitsFromUSD(team.MonthlyLimit)
		remainingMonth := capUnits - team.MonthlyConsumedUnits
		if remainingMonth < 0 {
			remainingMonth = 0
		}
		if remainingMonth < units {
			units = remainingMonth
		}
	}
	return units, nil
}

// PreConsumeTeamBalance pre-deducts quota units from team USD balance (transactional).
func PreConsumeTeamBalance(teamId uint, quotaUnits int) error {
	if quotaUnits <= 0 {
		return nil
	}
	usd := USDFromQuotaUnits(quotaUnits)
	return DB.Transaction(func(tx *gorm.DB) error {
		var team Team
		if err := tx.First(&team, teamId).Error; err != nil {
			return err
		}
		ym := currentBillingPeriodYM()
		monthlyBase := team.MonthlyConsumedUnits
		if team.BillingPeriodYm != ym {
			monthlyBase = 0
		}
		if team.Status != "active" {
			return errors.New("team is not active")
		}
		if team.Balance < usd {
			return errors.New("team balance insufficient")
		}
		if team.MonthlyLimit > 0 {
			capUnits := QuotaUnitsFromUSD(team.MonthlyLimit)
			if monthlyBase+quotaUnits > capUnits {
				return errors.New("team monthly limit exceeded")
			}
		}
		res := tx.Model(&Team{}).Where("id = ? AND balance >= ?", teamId, usd).
			Updates(map[string]interface{}{
				"balance":                gorm.Expr("balance - ?", usd),
				"used_quota":             gorm.Expr("used_quota + ?", quotaUnits),
				"monthly_consumed_units": monthlyBase + quotaUnits,
				"billing_period_ym":      ym,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("team balance insufficient")
		}
		return nil
	})
}

// DeltaTeamBalance adjusts team balance by quota units (positive = charge more, negative = refund).
func DeltaTeamBalance(teamId uint, deltaQuotaUnits int) error {
	if deltaQuotaUnits == 0 {
		return nil
	}
	usd := USDFromQuotaUnits(deltaQuotaUnits)
	if deltaQuotaUnits > 0 {
		return PreConsumeTeamBalance(teamId, deltaQuotaUnits)
	}
	// refund
	refund := -usd
	return DB.Model(&Team{}).Where("id = ?", teamId).
		Updates(map[string]interface{}{
			"balance":    gorm.Expr("balance + ?", refund),
			"used_quota": gorm.Expr("used_quota - ?", -deltaQuotaUnits),
		}).Error
}

// GetTeamAPIKeyRateLimit returns the rate limit (requests/min) for the given
// team API key string, or 0 if the key is not found or not active.
func GetTeamAPIKeyRateLimit(apiKey string) int {
	var key TeamAPIKey
	err := DB.Select("rate_limit").Where("key = ? AND status = ?", apiKey, "active").First(&key).Error
	if err != nil {
		return 0
	}
	return key.RateLimit
}
