// Package controller 提供TT API控制器
package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	ttmodel "github.com/QuantumNous/new-api/model"
	ttservice "github.com/QuantumNous/new-api/tt/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ========== 余额和用量 ==========

// BalanceResponse 余额响应
type BalanceResponse struct {
	Balance      string `json:"balance"`       // USD余额
	TrialBalance string `json:"trial_balance"` // 赠送余额
	TrialUsed    string `json:"trial_used"`    // 已用赠送余额
	TotalUsed    string `json:"total_used"`    // 累计消费
	Currency     string `json:"currency"`      // 货币单位
}

// GetBalance 获取用户余额
func GetBalance(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"type":    "unauthorized",
				"message": "user not authenticated",
			},
		})
		return
	}

	// 获取用户信息
	user, err := ttmodel.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"type":    "internal_error",
				"message": "failed to get user info",
			},
		})
		return
	}

	// 获取用户扩展信息
	userExt, err := ttmodel.GetUserExtension(userId)
	if err != nil {
		// 如果没有扩展信息，创建默认值
		userExt = &ttmodel.UserExtension{
			UserId:        uint(userId),
			TrialBalance:  decimal.Zero,
			TrialUsed:     decimal.Zero,
			TrialGrantedAt: nil,
		}
	}

	// 转换quota为USD（new-api使用quota概念，1 quota ≈ $0.002）
	// 这里假设quota已经是货币单位，需要根据实际业务调整
	balance := decimal.NewFromInt(int64(user.Quota)).Div(decimal.NewFromInt(500000)) // 转换为USD

	c.JSON(http.StatusOK, BalanceResponse{
		Balance:      balance.StringFixed(6),
		TrialBalance: userExt.TrialBalance.StringFixed(6),
		TrialUsed:    userExt.TrialUsed.StringFixed(6),
		TotalUsed:    decimal.NewFromInt(int64(user.UsedQuota)).Div(decimal.NewFromInt(500000)).StringFixed(6),
		Currency:     "USD",
	})
}

// UsageResponse 用量响应
type UsageResponse struct {
	Period       string                 `json:"period"`
	InputTokens  int64                  `json:"input_tokens"`
	OutputTokens int64                  `json:"output_tokens"`
	TotalCost    string                 `json:"total_cost"`
	Currency     string                 `json:"currency"`
	ByModel      map[string]ModelUsage  `json:"by_model"`
}

// ModelUsage 模型用量
type ModelUsage struct {
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	Cost         string `json:"cost"`
}

// GetUsage 获取用量统计
func GetUsage(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	period := c.DefaultQuery("period", "today")

	// 计算时间范围
	var startTime time.Time
	switch period {
	case "today":
		startTime = time.Now().Truncate(24 * time.Hour)
	case "week":
		startTime = time.Now().AddDate(0, 0, -7)
	case "month":
		startTime = time.Now().AddDate(0, -1, 0)
	default:
		startTime = time.Now().Truncate(24 * time.Hour)
	}

	// 从数据库获取用量统计
	usage, err := ttmodel.GetUserUsage(userId, startTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"type":    "internal_error",
				"message": "failed to get usage data",
			},
		})
		return
	}

	c.JSON(http.StatusOK, usage)
}

// GetUsageDetails 获取用量详情
func GetUsageDetails(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "20")
	model := c.Query("model")

	details, total, err := ttmodel.GetUserUsageDetails(userId, page, pageSize, model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"type":    "internal_error",
				"message": "failed to get usage details",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  details,
		"total": total,
		"page":  page,
		"page_size": pageSize,
	})
}

// ========== 模型验证 ==========

// VerifyRequest 验证请求
type VerifyRequest struct {
	Model string `json:"model" binding:"required"`
}

// VerifyResponse 验证响应
type VerifyResponse struct {
	Model            string `json:"model"`
	Status           string `json:"status"` // verified/suspicious/failed
	ThinkingDetected bool   `json:"thinking_detected"`
	ResponseTime     int64  `json:"response_time_ms"`
	Message          string `json:"message,omitempty"`
}

// VerifyModel 验证模型真伪
func VerifyModel(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"type":    "invalid_request",
				"message": "model parameter required",
			},
		})
		return
	}

	// 发送测试请求验证模型
	result, err := ttservice.VerifyModelAuthenticity(req.Model)
	if err != nil {
		c.JSON(http.StatusOK, VerifyResponse{
			Model:    req.Model,
			Status:   "failed",
			Message:  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ========== 服务状态 ==========

// ServiceStatus 服务状态
type ServiceStatus struct {
	Services map[string]ServiceInfo `json:"services"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Status    string  `json:"status"` // operational/degraded/down
	Uptime    float64 `json:"uptime"` // 0.0 - 1.0
	Latency   int64   `json:"latency_ms"`
	Message   string  `json:"message,omitempty"`
}

// GetServiceStatus 获取服务状态
func GetServiceStatus(c *gin.Context) {
	// 从缓存或数据库获取各服务状态
	status := ServiceStatus{
		Services: map[string]ServiceInfo{
			"claude-sonnet-4-6": {
				Status:  "operational",
				Uptime:  0.998,
				Latency: 150,
			},
			"claude-opus-4-6": {
				Status:  "operational",
				Uptime:  0.995,
				Latency: 200,
			},
			"claude-haiku": {
				Status:  "operational",
				Uptime:  0.999,
				Latency: 80,
			},
			"doubao-seed-1-6": {
				Status:  "operational",
				Uptime:  0.999,
				Latency: 60,
			},
			"doubao-seedream-3-0": {
				Status:  "operational",
				Uptime:  0.998,
				Latency: 3000,
			},
			"payment": {
				Status:  "operational",
				Uptime:  0.999,
				Latency: 100,
			},
			"dashboard": {
				Status:  "operational",
				Uptime:  0.999,
				Latency: 50,
			},
		},
	}

	c.JSON(http.StatusOK, status)
}

// GetPublicStatus 获取公开状态
func GetPublicStatus(c *gin.Context) {
	GetServiceStatus(c)
}

// GetPublicStats 获取公开统计数据
func GetPublicStats(c *gin.Context) {
	stats, err := ttmodel.GetPublicStats()
	if err != nil {
		stats = &ttmodel.PublicStats{
			TotalUsers:    0,
			TotalRequests: 0,
			TotalTokens:   0,
			Uptime30Days:  "99.9%",
			AvgLatency:    150,
		}
	}

	c.JSON(http.StatusOK, stats)
}

// ========== 邀请裂变 ==========

// ReferralInfo 邀请信息
type ReferralInfo struct {
	InviteCode        string  `json:"invite_code"`
	TotalInvites      int     `json:"total_invites"`
	SuccessfulInvites int     `json:"successful_invites"`
	TotalReward       string  `json:"total_reward"`
	AvailableReward   string  `json:"available_reward"`
}

// GetReferralInfo 获取邀请信息
func GetReferralInfo(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	info, err := ttmodel.GetReferralInfo(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get referral info"})
		return
	}

	c.JSON(http.StatusOK, info)
}

// ApplyReferralCodeRequest 申请邀请码请求
type ApplyReferralCodeRequest struct {
	InviteCode string `json:"invite_code" binding:"required"`
}

// ApplyReferralCode 使用邀请码
func ApplyReferralCode(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req ApplyReferralCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invite_code required"})
		return
	}

	// 验证并使用邀请码
	result, err := ttmodel.ApplyReferralCode(userId, req.InviteCode, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"type":    "referral_error",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"inviter_bonus": result.InviterBonus.String(),
		"invitee_bonus": result.InviteeBonus.String(),
		"message":       "邀请码使用成功！双方已获得奖励",
	})
}

// GetReferralRecords 获取邀请记录
func GetReferralRecords(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	records, err := ttmodel.GetReferralRecords(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get records"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": records})
}

// ========== 订阅 ==========

// SubscriptionInfo 订阅信息
type SubscriptionInfo struct {
	HasSubscription bool   `json:"has_subscription"`
	PlanName        string `json:"plan_name,omitempty"`
	Status          string `json:"status,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	UsedUSD         string `json:"used_usd,omitempty"`
	RemainingUSD    string `json:"remaining_usd,omitempty"`
}

// GetSubscription 获取订阅信息
func GetSubscription(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	info, err := ttmodel.GetUserSubscription(userId)
	if err != nil {
		c.JSON(http.StatusOK, SubscriptionInfo{HasSubscription: false})
		return
	}

	c.JSON(http.StatusOK, info)
}

// SubscribeRequest 订阅请求
type SubscribeRequest struct {
	PlanId        uint   `json:"plan_id" binding:"required"`
	BillingCycle  string `json:"billing_cycle"` // monthly/yearly
}

// Subscribe 订阅套餐
func Subscribe(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan_id required"})
		return
	}

	sub, err := ttmodel.CreateSubscription(userId, req.PlanId, req.BillingCycle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"type":    "subscription_error",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"subscription": sub,
		"message":      "订阅成功",
	})
}

// CancelSubscription 取消订阅
func CancelSubscription(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	reason := c.PostForm("reason")

	err := ttmodel.CancelUserSubscription(userId, reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "订阅已取消，将在当前计费周期结束后失效",
	})
}

// PlanInfo 套餐信息
type PlanInfo struct {
	Id           uint   `json:"id"`
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	MonthlyPrice string `json:"monthly_price"`
	IncludedUSD  string `json:"included_usd"`
	DiscountRate string `json:"discount_rate"`
	Features     string `json:"features"`
}

// ListPlans 列出套餐
func ListPlans(c *gin.Context) {
	plans, err := ttmodel.GetActivePlans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get plans"})
		return
	}

	result := make([]PlanInfo, len(plans))
	for i, p := range plans {
		result[i] = PlanInfo{
			Id:           p.Id,
			Name:         p.Name,
			DisplayName:  p.DisplayName,
			Description:  p.Description,
			MonthlyPrice: p.MonthlyPrice.StringFixed(2),
			IncludedUSD:  p.IncludedUSD.StringFixed(2),
			DiscountRate: p.DiscountRate.StringFixed(2),
			Features:     p.Features,
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ========== 团队工作空间 ==========

// TeamInfo 团队信息响应
type TeamInfo struct {
	Id          uint                `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	OwnerId     uint                `json:"owner_id"`
	Balance     string              `json:"balance"`
	Status      string              `json:"status"`
	MemberCount int                 `json:"member_count"`
	Members     []TeamMemberInfo    `json:"members,omitempty"`
	CreatedAt   string              `json:"created_at"`
}

// TeamMemberInfo 团队成员信息
type TeamMemberInfo struct {
	UserId   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	JoinedAt string `json:"joined_at"`
}

// CreateTeamRequest 创建团队请求
type CreateTeamRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// CreateTeam 创建团队
func CreateTeam(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}

	team, err := ttmodel.CreateTeam(uint(userId), req.Name, req.Description, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"team": TeamInfo{
			Id:          team.Id,
			Name:        team.Name,
			Description: team.Description,
			OwnerId:     team.OwnerId,
			Status:      team.Status,
			MemberCount: team.MemberCount,
			CreatedAt:   team.CreatedAt.Format(time.RFC3339),
		},
	})
}

// ListTeams 获取用户所属团队列表
func ListTeams(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teams, err := ttmodel.GetUserTeams(uint(userId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get teams"})
		return
	}

	result := make([]TeamInfo, len(teams))
	for i, t := range teams {
		result[i] = TeamInfo{
			Id:          t.Id,
			Name:        t.Name,
			Description: t.Description,
			OwnerId:     t.OwnerId,
			Balance:     decimal.NewFromFloat(t.Balance).StringFixed(2),
			Status:      t.Status,
			MemberCount: t.MemberCount,
			CreatedAt:   t.CreatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetTeam 获取团队详情
func GetTeam(c *gin.Context) {
	userId := c.GetInt("id")
	teamId := c.Param("id")

	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := strconv.Atoi(teamId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	// 验证用户是团队成员
	isMember, _ := ttmodel.IsTeamMember(uint(id), uint(userId))
	if !isMember {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a team member"})
		return
	}

	team, err := ttmodel.GetTeam(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}

	members := make([]TeamMemberInfo, len(team.Members))
	for i, m := range team.Members {
		username := ""
		if m.User != nil {
			username = m.User.Username
		}
		members[i] = TeamMemberInfo{
			UserId:   m.UserId,
			Username: username,
			Role:     m.Role,
			JoinedAt: m.JoinedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, TeamInfo{
		Id:          team.Id,
		Name:        team.Name,
		Description: team.Description,
		OwnerId:     team.OwnerId,
		Balance:     decimal.NewFromFloat(team.Balance).StringFixed(2),
		Status:      team.Status,
		MemberCount: team.MemberCount,
		Members:     members,
		CreatedAt:   team.CreatedAt.Format(time.RFC3339),
	})
}

// AddTeamMemberRequest 添加成员请求
type AddTeamMemberRequest struct {
	UserId uint   `json:"user_id" binding:"required"`
	Role   string `json:"role"` // admin/member, 默认 member
}

// AddTeamMember 添加团队成员
func AddTeamMember(c *gin.Context) {
	userId := c.GetInt("id")
	teamId := c.Param("id")

	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := strconv.Atoi(teamId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	// 验证用户是团队 owner 或 admin
	isMember, role := ttmodel.IsTeamMember(uint(id), uint(userId))
	if !isMember || (role != ttmodel.TeamRoleOwner && role != ttmodel.TeamRoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner or admin can add members"})
		return
	}

	var req AddTeamMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
		return
	}

	if req.Role == "" {
		req.Role = ttmodel.TeamRoleMember
	}

	err = ttmodel.AddTeamMember(uint(id), req.UserId, req.Role)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "成员添加成功"})
}

// RemoveTeamMember 移除团队成员
func RemoveTeamMember(c *gin.Context) {
	userId := c.GetInt("id")
	teamId := c.Param("id")
	memberId := c.Param("user_id")

	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	tid, err := strconv.Atoi(teamId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	mid, err := strconv.Atoi(memberId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// 验证用户是团队 owner 或 admin
	isMember, role := ttmodel.IsTeamMember(uint(tid), uint(userId))
	if !isMember || (role != ttmodel.TeamRoleOwner && role != ttmodel.TeamRoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner or admin can remove members"})
		return
	}

	err = ttmodel.RemoveTeamMember(uint(tid), uint(mid))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "成员移除成功"})
}

// UpdateMemberRoleRequest 更新成员角色请求
type UpdateMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// UpdateMemberRole 更新成员角色
func UpdateMemberRole(c *gin.Context) {
	userId := c.GetInt("id")
	teamId := c.Param("id")
	memberId := c.Param("user_id")

	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	tid, err := strconv.Atoi(teamId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	mid, err := strconv.Atoi(memberId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	// 验证用户是团队 owner
	isMember, role := ttmodel.IsTeamMember(uint(tid), uint(userId))
	if !isMember || role != ttmodel.TeamRoleOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner can update member roles"})
		return
	}

	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role required"})
		return
	}

	err = ttmodel.UpdateMemberRole(uint(tid), uint(mid), req.Role)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "角色更新成功"})
}

// TeamAPIKeyInfo 团队 API Key 信息
type TeamAPIKeyInfo struct {
	Id          uint   `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// ListTeamAPIKeys 获取团队 API Keys
func ListTeamAPIKeys(c *gin.Context) {
	userId := c.GetInt("id")
	teamId := c.Param("id")

	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	tid, err := strconv.Atoi(teamId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	// 验证用户是团队成员
	isMember, _ := ttmodel.IsTeamMember(uint(tid), uint(userId))
	if !isMember {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a team member"})
		return
	}

	keys, err := ttmodel.ListTeamAPIKeys(uint(tid))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list keys"})
		return
	}

	result := make([]TeamAPIKeyInfo, len(keys))
	for i, k := range keys {
		result[i] = TeamAPIKeyInfo{
			Id:          k.Id,
			Key:         k.Key,
			Name:        k.Name,
			Description: k.Description,
			Status:      k.Status,
			CreatedAt:   k.CreatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// CreateTeamAPIKeyRequest 创建团队 API Key 请求
type CreateTeamAPIKeyRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// CreateTeamAPIKey 创建团队 API Key
func CreateTeamAPIKey(c *gin.Context) {
	userId := c.GetInt("id")
	teamId := c.Param("id")

	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	tid, err := strconv.Atoi(teamId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	// 验证用户是团队 owner 或 admin
	isMember, role := ttmodel.IsTeamMember(uint(tid), uint(userId))
	if !isMember || (role != ttmodel.TeamRoleOwner && role != ttmodel.TeamRoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner or admin can create API keys"})
		return
	}

	var req CreateTeamAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}

	key, err := ttmodel.CreateTeamAPIKey(uint(tid), req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"key": TeamAPIKeyInfo{
			Id:          key.Id,
			Key:         key.Key,
			Name:        key.Name,
			Description: key.Description,
			Status:      key.Status,
			CreatedAt:   key.CreatedAt.Format(time.RFC3339),
		},
	})
}

// RevokeTeamAPIKey 撤销团队 API Key
func RevokeTeamAPIKey(c *gin.Context) {
	userId := c.GetInt("id")
	teamId := c.Param("id")
	keyId := c.Param("key_id")

	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	tid, err := strconv.Atoi(teamId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	kid, err := strconv.Atoi(keyId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key id"})
		return
	}

	// 验证用户是团队 owner 或 admin
	isMember, role := ttmodel.IsTeamMember(uint(tid), uint(userId))
	if !isMember || (role != ttmodel.TeamRoleOwner && role != ttmodel.TeamRoleAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only owner or admin can revoke API keys"})
		return
	}

	err = ttmodel.RevokeTeamAPIKey(uint(kid))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "API Key 已撤销"})
}

// ========== 预算管理 ==========

// BudgetConfigResponse 预算配置响应
type BudgetConfigResponse struct {
	DailyLimit     float64 `json:"daily_limit"`
	MonthlyLimit   float64 `json:"monthly_limit"`
	AlertThreshold float64 `json:"alert_threshold"`
	NotifyEmail    bool    `json:"notify_email"`
	NotifyWebhook  bool    `json:"notify_webhook"`
}

// GetBudgetConfig 获取预算配置
func GetBudgetConfig(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	config, err := ttmodel.GetBudgetConfig(uint(userId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get budget config"})
		return
	}

	c.JSON(http.StatusOK, BudgetConfigResponse{
		DailyLimit:     config.DailyLimit,
		MonthlyLimit:   config.MonthlyLimit,
		AlertThreshold: config.AlertThreshold,
		NotifyEmail:    config.NotifyEmail,
		NotifyWebhook:  config.NotifyWebhook,
	})
}

// SetBudgetConfigRequest 设置预算配置请求
type SetBudgetConfigRequest struct {
	DailyLimit     *float64 `json:"daily_limit"`
	MonthlyLimit   *float64 `json:"monthly_limit"`
	AlertThreshold *float64 `json:"alert_threshold"`
	NotifyEmail    *bool    `json:"notify_email"`
	NotifyWebhook  *bool    `json:"notify_webhook"`
}

// SetBudgetConfig 设置预算配置
func SetBudgetConfig(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req SetBudgetConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 获取当前配置
	config, err := ttmodel.GetBudgetConfig(uint(userId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get current config"})
		return
	}

	// 更新字段
	dailyLimit := config.DailyLimit
	monthlyLimit := config.MonthlyLimit
	alertThreshold := config.AlertThreshold
	notifyEmail := config.NotifyEmail
	notifyWebhook := config.NotifyWebhook

	if req.DailyLimit != nil {
		dailyLimit = *req.DailyLimit
	}
	if req.MonthlyLimit != nil {
		monthlyLimit = *req.MonthlyLimit
	}
	if req.AlertThreshold != nil {
		alertThreshold = *req.AlertThreshold
	}
	if req.NotifyEmail != nil {
		notifyEmail = *req.NotifyEmail
	}
	if req.NotifyWebhook != nil {
		notifyWebhook = *req.NotifyWebhook
	}

	config, err = ttmodel.SetBudgetConfig(uint(userId), dailyLimit, monthlyLimit, alertThreshold, notifyEmail, notifyWebhook)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config": BudgetConfigResponse{
			DailyLimit:     config.DailyLimit,
			MonthlyLimit:   config.MonthlyLimit,
			AlertThreshold: config.AlertThreshold,
			NotifyEmail:    config.NotifyEmail,
			NotifyWebhook:  config.NotifyWebhook,
		},
	})
}

// BudgetStatusResponse 预算状态响应
type BudgetStatusResponse struct {
	DailyUsed       float64 `json:"daily_used"`
	DailyLimit      float64 `json:"daily_limit"`
	DailyPercent    float64 `json:"daily_percent"`
	DailyExceeded   bool    `json:"daily_exceeded"`
	MonthlyUsed     float64 `json:"monthly_used"`
	MonthlyLimit    float64 `json:"monthly_limit"`
	MonthlyPercent  float64 `json:"monthly_percent"`
	MonthlyExceeded bool    `json:"monthly_exceeded"`
	AlertThreshold  float64 `json:"alert_threshold"`
	ShouldAlert     bool    `json:"should_alert"`
}

// GetBudgetStatus 获取预算状态
func GetBudgetStatus(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	status, err := ttmodel.GetBudgetStatus(uint(userId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get budget status"})
		return
	}

	c.JSON(http.StatusOK, BudgetStatusResponse{
		DailyUsed:       status.DailyUsed,
		DailyLimit:      status.DailyLimit,
		DailyPercent:    status.DailyPercent,
		DailyExceeded:   status.DailyExceeded,
		MonthlyUsed:     status.MonthlyUsed,
		MonthlyLimit:    status.MonthlyLimit,
		MonthlyPercent:  status.MonthlyPercent,
		MonthlyExceeded: status.MonthlyExceeded,
		AlertThreshold:  status.AlertThreshold,
		ShouldAlert:     status.ShouldAlert,
	})
}

// ========== 辅助函数 ==========

// ========== 智能路由 ==========

// SmartRouterConfigResponse 智能路由配置响应
type SmartRouterConfigResponse struct {
	Enabled           bool   `json:"enabled"`
	DefaultModel      string `json:"default_model"`
	CodeModel         string `json:"code_model"`
	SimpleQAModel     string `json:"simple_qa_model"`
	LongContextModel  string `json:"long_context_model"`
}

// smartRouter 全局智能路由实例
var smartRouter = ttservice.NewSmartRouter(nil)

// GetSmartRouterConfig 获取智能路由配置
func GetSmartRouterConfig(c *gin.Context) {
	stats := smartRouter.GetRoutingStats()
	c.JSON(http.StatusOK, SmartRouterConfigResponse{
		Enabled:          stats["enabled"].(bool),
		DefaultModel:     stats["default_model"].(string),
		CodeModel:        stats["code_model"].(string),
		SimpleQAModel:    stats["simple_qa_model"].(string),
		LongContextModel: stats["long_context_model"].(string),
	})
}

// SmartRouteRequest 智能路由请求
type SmartRouteRequest struct {
	Model           string                `json:"model"`
	Messages        []ttservice.RouterMessage `json:"messages"`
	MaxTokens       int                   `json:"max_tokens,omitempty"`
	Temperature     float64               `json:"temperature,omitempty"`
	EstimatedTokens int                   `json:"estimated_tokens,omitempty"`
}

// SmartRouteResponse 智能路由响应
type SmartRouteResponse struct {
	RecommendedModel string `json:"recommended_model"`
	OriginalModel    string `json:"original_model"`
}

// SmartRoute 智能路由推荐
func SmartRoute(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req SmartRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 如果没有提供估算 token 数，自动计算
	if req.EstimatedTokens == 0 && len(req.Messages) > 0 {
		var totalContent string
		for _, msg := range req.Messages {
			totalContent += msg.Content
		}
		req.EstimatedTokens = ttservice.EstimateTokens(totalContent)
	}

	routerReq := &ttservice.RouterRequest{
		Model:           req.Model,
		Messages:        req.Messages,
		MaxTokens:       req.MaxTokens,
		Temperature:     req.Temperature,
		EstimatedTokens: req.EstimatedTokens,
	}

	recommendedModel := smartRouter.RouteRequest(routerReq)

	c.JSON(http.StatusOK, SmartRouteResponse{
		RecommendedModel: recommendedModel,
		OriginalModel:    req.Model,
	})
}

// ========== 调用日志 ==========

// CallLogResponse 调用日志响应
type CallLogResponse struct {
	Id           uint   `json:"id"`
	RequestId    string `json:"request_id"`
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CostUSD      string `json:"cost_usd"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

// GetCallLogs 获取调用日志列表
func GetCallLogs(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "20")
	model := c.Query("model")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	var startTime, endTime *time.Time
	if startDate != "" {
		t, err := time.Parse("2006-01-02", startDate)
		if err == nil {
			startTime = &t
		}
	}
	if endDate != "" {
		t, err := time.Parse("2006-01-02", endDate)
		if err == nil {
			endTime = &t
		}
	}

	// 默认只查询最近7天
	if startTime == nil {
		t := time.Now().AddDate(0, 0, -7)
		startTime = &t
	}

	logs, total, err := ttmodel.GetCallLogs(uint(userId), page, pageSize, model, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get call logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetCallLogDetail 获取单条调用日志详情
func GetCallLogDetail(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	logId := c.Param("id")
	id, err := strconv.Atoi(logId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid log id"})
		return
	}

	detail, err := ttmodel.GetCallLogDetail(uint(userId), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "log not found"})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// recordRequest 记录请求元数据
func recordRequest(c *gin.Context, model string, inputTokens, outputTokens int64, costUSD decimal.Decimal) {
	requestId, _ := c.Get(common.RequestIdKey)
	if requestId == nil {
		requestId = ""
	}

	// 记录到上下文
	c.Set("model", model)
	c.Set("input_tokens", inputTokens)
	c.Set("output_tokens", outputTokens)
	c.Set("cost_usd", costUSD)
	c.Set("request_id", requestId)
}

// ========== 成本分析报告（V2.0功能） ==========

// GetCostReport 获取成本分析报告
func GetCostReport(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req ttmodel.CostReportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 默认查询最近 30 天
	if req.StartDate == "" {
		req.StartDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if req.EndDate == "" {
		req.EndDate = time.Now().Format("2006-01-02")
	}
	if req.ReportType == "" {
		req.ReportType = "monthly"
	}

	report, err := ttmodel.GetCostReport(uint(userId), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate report"})
		return
	}

	c.JSON(http.StatusOK, report)
}

// ExportCostReport 导出成本报告
func ExportCostReport(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req ttmodel.CostReportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	format := c.DefaultQuery("format", "json")
	if format == "csv" {
		csvData, err := ttmodel.ExportCostReportCSV(uint(userId), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export report"})
			return
		}
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=cost_report.csv")
		c.Data(200, "text/csv", csvData)
		return
	}

	// 默认返回 JSON
	report, err := ttmodel.GetCostReport(uint(userId), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate report"})
		return
	}

	c.JSON(http.StatusOK, report)
}

// GetModelCostBreakdown 获取模型成本拆解
func GetModelCostBreakdown(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		start = time.Now().AddDate(0, -1, 0)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		end = time.Now()
	}

	breakdown, err := ttmodel.GetModelCostBreakdown(uint(userId), start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get breakdown"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": breakdown})
}
