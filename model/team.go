//go:build tt
// +build tt

package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

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
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// ListTeamAPIKeys 列出团队API Key
func ListTeamAPIKeys(teamId uint) ([]TeamAPIKey, error) {
	var keys []TeamAPIKey
	err := DB.Where("team_id = ?", teamId).Find(&keys).Error
	return keys, err
}

// RevokeTeamAPIKey 撤销团队API Key
func RevokeTeamAPIKey(keyId uint) error {
	return DB.Model(&TeamAPIKey{}).Where("id = ?", keyId).
		Update("status", "revoked").Error
}
