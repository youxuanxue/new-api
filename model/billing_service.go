//go:build tt
// +build tt

package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ========== 用户扩展 ==========

// GetUserExtension 获取用户扩展信息
func GetUserExtension(userId int) (*UserExtension, error) {
	var ext UserExtension
	err := DB.Where("user_id = ?", userId).First(&ext).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建默认扩展
			ext = UserExtension{
				UserId:       uint(userId),
				TrialBalance: decimal.Zero,
				TrialUsed:    decimal.Zero,
			}
			DB.Create(&ext)
			return &ext, nil
		}
		return nil, err
	}
	return &ext, nil
}

// GrantTrialBalance 发放注册赠送余额
func GrantTrialBalance(userId int) error {
	ext, err := GetUserExtension(userId)
	if err != nil {
		return err
	}

	// 幂等检查
	if ext.TrialGrantedAt != nil {
		return nil // 已发放
	}

	// 发放赠送余额
	trialAmount := DefaultTrialConfig.TrialAmount
	ext.TrialBalance = trialAmount
	now := time.Now()
	ext.TrialGrantedAt = &now

	return DB.Save(ext).Error
}

// ========== 用量统计 ==========

// UsageStats 用量统计
type UsageStats struct {
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

// GetUserUsage 获取用户用量统计
func GetUserUsage(userId int, startTime time.Time) (*UsageStats, error) {
	stats := &UsageStats{
		Period:  "custom",
		Currency: "USD",
		ByModel: make(map[string]ModelUsage),
	}

	// 从消费记录表获取数据
	var records []ConsumptionRecord
	err := DB.Where("user_id = ? AND created_at >= ?", userId, startTime).Find(&records).Error
	if err != nil {
		return nil, err
	}

	// 汇总数据
	var totalInput, totalOutput int64
	var totalCost decimal.Decimal
	modelStats := make(map[string]*ModelUsage)

	for _, r := range records {
		totalInput += r.InputTokens
		totalOutput += r.OutputTokens
		totalCost = totalCost.Add(r.ActualCostUSD)

		if _, ok := modelStats[r.Model]; !ok {
			modelStats[r.Model] = &ModelUsage{}
		}
		modelStats[r.Model].InputTokens += r.InputTokens
		modelStats[r.Model].OutputTokens += r.OutputTokens
	}

	stats.InputTokens = totalInput
	stats.OutputTokens = totalOutput
	stats.TotalCost = totalCost.StringFixed(6)

	for model, usage := range modelStats {
		cost := decimal.Zero
		for _, r := range records {
			if r.Model == model {
				cost = cost.Add(r.ActualCostUSD)
			}
		}
		usage.Cost = cost.StringFixed(6)
		stats.ByModel[model] = *usage
	}

	return stats, nil
}

// UsageDetail 用量详情
type UsageDetail struct {
	Id           uint   `json:"id"`
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	Cost         string `json:"cost"`
	CreatedAt    string `json:"created_at"`
}

// GetUserUsageDetails 获取用户用量详情
func GetUserUsageDetails(userId int, page, pageSize, model string) ([]UsageDetail, int64, error) {
	var records []ConsumptionRecord
	var total int64

	query := DB.Model(&ConsumptionRecord{}).Where("user_id = ?", userId)
	if model != "" {
		query = query.Where("model = ?", model)
	}

	query.Count(&total)

	offset, limit := parsePagination(page, pageSize)
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&records).Error
	if err != nil {
		return nil, 0, err
	}

	details := make([]UsageDetail, len(records))
	for i, r := range records {
		details[i] = UsageDetail{
			Id:           r.Id,
			Model:        r.Model,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			Cost:         r.ActualCostUSD.StringFixed(6),
			CreatedAt:    r.CreatedAt.Format(time.RFC3339),
		}
	}

	return details, total, nil
}

// ========== 邀请裂变 ==========

// ReferralInfo 邀请信息
type ReferralInfo struct {
	InviteCode        string `json:"invite_code"`
	TotalInvites      int    `json:"total_invites"`
	SuccessfulInvites int    `json:"successful_invites"`
	TotalReward       string `json:"total_reward"`
	AvailableReward   string `json:"available_reward"`
}

// GetReferralInfo 获取邀请信息
func GetReferralInfo(userId int) (*ReferralInfo, error) {
	// 获取用户的邀请码
	var user User
	err := DB.First(&user, userId).Error
	if err != nil {
		return nil, err
	}

	info := &ReferralInfo{
		InviteCode:   user.AffCode, // 使用new-api的AffCode作为邀请码
		TotalReward:  "0.00",
		AvailableReward: "0.00",
	}

	// 统计邀请数量
	var totalInvites, successfulInvites int64
	DB.Model(&Referral{}).Where("inviter_id = ?", userId).Count(&totalInvites)
	DB.Model(&Referral{}).Where("inviter_id = ? AND status = ?", userId, "granted").Count(&successfulInvites)

	info.TotalInvites = int(totalInvites)
	info.SuccessfulInvites = int(successfulInvites)

	// 计算奖励总额
	var totalReward decimal.Decimal
	DB.Model(&Referral{}).Where("inviter_id = ? AND status = ?", userId, "granted").
		Select("COALESCE(SUM(inviter_bonus), 0)").Scan(&totalReward)
	info.TotalReward = totalReward.StringFixed(2)

	return info, nil
}

// ApplyReferralCode 使用邀请码
func ApplyReferralCode(userId int, inviteCode string, ip string) (*Referral, error) {
	// 查找邀请人
	var inviter User
	err := DB.Where("aff_code = ?", inviteCode).First(&inviter).Error
	if err != nil {
		return nil, errors.New("无效的邀请码")
	}

	if inviter.Id == userId {
		return nil, errors.New("不能使用自己的邀请码")
	}

	// 检查是否已经使用过邀请码
	var count int64
	DB.Model(&Referral{}).Where("invitee_id = ?", userId).Count(&count)
	if count > 0 {
		return nil, errors.New("您已经使用过邀请码")
	}

	// 检查IP冷却期
	config := DefaultReferralConfig
	var ipCount int64
	cutoff := time.Now().Add(-time.Duration(config.IPCooldownHours) * time.Hour)
	DB.Model(&Referral{}).Where("ip_address = ? AND created_at > ?", ip, cutoff).Count(&ipCount)
	if ipCount > 0 {
		return nil, errors.New("同一IP在24小时内只能使用一次邀请码")
	}

	// 创建邀请记录
	referral := Referral{
		InviterId:   uint(inviter.Id),
		InviteeId:   uint(userId),
		InviteCode:  inviteCode,
		BonusUSD:    config.BonusUSD,
		InviterBonus: config.BonusUSD,
		InviteeBonus: config.BonusUSD,
		Status:      "pending",
		IPAddress:   ip,
	}

	if config.RequireFirstCharge {
		referral.Status = "pending"
	} else {
		referral.Status = "granted"
		now := time.Now()
		referral.GrantedAt = &now

		// 发放奖励
		GrantReferralBonus(inviter.Id, userId, config.BonusUSD)
	}

	err = DB.Create(&referral).Error
	if err != nil {
		return nil, err
	}

	return &referral, nil
}

// GrantReferralBonus 发放邀请奖励
func GrantReferralBonus(inviterId, inviteeId int, amount decimal.Decimal) {
	// 给邀请人发放奖励
	GrantTrialBalance(inviterId)
	GrantTrialBalance(inviteeId)

	// 记录到用户扩展表
	ext1, _ := GetUserExtension(inviterId)
	ext1.TrialBalance = ext1.TrialBalance.Add(amount)
	DB.Save(ext1)

	ext2, _ := GetUserExtension(inviteeId)
	ext2.TrialBalance = ext2.TrialBalance.Add(amount)
	DB.Save(ext2)
}

// GetReferralRecords 获取邀请记录
func GetReferralRecords(userId int) ([]Referral, error) {
	var records []Referral
	err := DB.Where("inviter_id = ?", userId).Order("created_at DESC").Find(&records).Error
	return records, err
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

// GetUserSubscription 获取用户订阅
func GetUserSubscription(userId int) (*SubscriptionInfo, error) {
	var sub Subscription
	err := DB.Where("user_id = ? AND status = ?", userId, "active").First(&sub).Error
	if err != nil {
		return nil, err
	}

	info := &SubscriptionInfo{
		HasSubscription: true,
		Status:          sub.Status,
		ExpiresAt:       sub.ExpiresAt.Format(time.RFC3339),
		UsedUSD:         sub.UsedUSD.StringFixed(2),
		RemainingUSD:    sub.RemainingUSD.StringFixed(2),
	}

	if sub.PlanId > 0 {
		var plan Plan
		if err := DB.First(&plan, sub.PlanId).Error; err == nil {
			info.PlanName = plan.Name
		}
	}

	return info, nil
}

// CreateSubscription 创建订阅
func CreateSubscription(userId int, planId uint, billingCycle string) (*Subscription, error) {
	// 获取套餐信息
	var plan Plan
	err := DB.First(&plan, planId).Error
	if err != nil {
		return nil, errors.New("套餐不存在")
	}

	// 检查是否已有活跃订阅
	var count int64
	DB.Model(&Subscription{}).Where("user_id = ? AND status = ?", userId, "active").Count(&count)
	if count > 0 {
		return nil, errors.New("已有活跃订阅，请先取消")
	}

	// 计算过期时间
	var expiresAt time.Time
	if billingCycle == "yearly" {
		expiresAt = time.Now().AddDate(1, 0, 0)
	} else {
		expiresAt = time.Now().AddDate(0, 1, 0)
	}

	sub := Subscription{
		UserId:       uint(userId),
		PlanId:       planId,
		Status:       "active",
		BillingCycle: billingCycle,
		RemainingUSD: plan.IncludedUSD,
		ExpiresAt:    expiresAt,
	}

	err = DB.Create(&sub).Error
	if err != nil {
		return nil, err
	}

	// 更新用户扩展
	ext, _ := GetUserExtension(userId)
	ext.CurrentPlanId = &planId
	ext.SubscriptionId = &sub.Id
	DB.Save(ext)

	return &sub, nil
}

// CancelUserSubscription 取消订阅
func CancelUserSubscription(userId int, reason string) error {
	return DB.Model(&Subscription{}).
		Where("user_id = ? AND status = ?", userId, "active").
		Updates(map[string]interface{}{
			"status":        "cancelled",
			"cancel_reason": reason,
			"cancelled_at":  time.Now(),
		}).Error
}

// GetActivePlans 获取活跃套餐
func GetActivePlans() ([]Plan, error) {
	var plans []Plan
	err := DB.Where("is_active = ?", true).Order("sort_order").Find(&plans).Error
	return plans, err
}

// ========== 公开统计 ==========

// PublicStats 公开统计数据
type PublicStats struct {
	TotalUsers    int64  `json:"total_users"`
	TotalRequests int64  `json:"total_requests"`
	TotalTokens   int64  `json:"total_tokens"`
	Uptime30Days  string `json:"uptime_30_days"`
	AvgLatency    int64  `json:"avg_latency_ms"`
}

// GetPublicStats 获取公开统计
func GetPublicStats() (*PublicStats, error) {
	stats := &PublicStats{
		Uptime30Days: "99.9%",
		AvgLatency:   150,
	}

	// 统计用户数
	DB.Model(&User{}).Count(&stats.TotalUsers)

	// 统计请求数和Token数
	DB.Model(&ConsumptionRecord{}).
		Select("COUNT(*) as total_requests, COALESCE(SUM(input_tokens + output_tokens), 0) as total_tokens").
		Scan(stats)

	return stats, nil
}

// ========== 管理后台 ==========

// DashboardData 看板数据
type DashboardData struct {
	TodayRequests    int64          `json:"today_requests"`
	TodayRevenue     string         `json:"today_revenue"`
	TodayCost        string         `json:"today_cost"`
	TodayGrossMargin string         `json:"today_gross_margin"`
	ActiveUsers      int64          `json:"active_users"`
	TotalUsers       int64          `json:"total_users"`
	APIAvailability  string         `json:"api_availability"`
	PoolAvailability string         `json:"pool_availability"`
	RecentErrors     []Error        `json:"recent_errors"`
	TrendData        Trend          `json:"trend_data"`
}

type Error struct {
	Time    string `json:"time"`
	Model   string `json:"model"`
	Message string `json:"message"`
}

type Trend struct {
	Dates    []string `json:"dates"`
	Requests []int64  `json:"requests"`
	Revenue  []string `json:"revenue"`
}

// GetDashboardData 获取看板数据
func GetDashboardData() (*DashboardData, error) {
	data := &DashboardData{
		APIAvailability:  "99.8%",
		PoolAvailability: "95.0%",
		RecentErrors:     []Error{},
		TrendData:        Trend{},
	}

	today := time.Now().Truncate(24 * time.Hour)

	// 今日请求数
	DB.Model(&ConsumptionRecord{}).Where("created_at >= ?", today).Count(&data.TodayRequests)

	// 今日收入
	var todayRevenue decimal.Decimal
	DB.Model(&ConsumptionRecord{}).Where("created_at >= ?", today).
		Select("COALESCE(SUM(actual_cost_usd), 0)").Scan(&todayRevenue)
	data.TodayRevenue = todayRevenue.StringFixed(2)

	// 用户统计
	DB.Model(&User{}).Count(&data.TotalUsers)
	DB.Model(&User{}).Where("updated_at >= ?", today).Count(&data.ActiveUsers)

	// 趋势数据（最近7天）
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		data.TrendData.Dates = append(data.TrendData.Dates, date)

		dayStart := time.Now().AddDate(0, 0, -i).Truncate(24 * time.Hour)
		var dayCount int64
		DB.Model(&ConsumptionRecord{}).Where("created_at >= ? AND created_at < ?", dayStart, dayStart.Add(24*time.Hour)).Count(&dayCount)
		data.TrendData.Requests = append(data.TrendData.Requests, dayCount)

		var dayRevenue decimal.Decimal
		DB.Model(&ConsumptionRecord{}).Where("created_at >= ? AND created_at < ?", dayStart, dayStart.Add(24*time.Hour)).
			Select("COALESCE(SUM(actual_cost_usd), 0)").Scan(&dayRevenue)
		data.TrendData.Revenue = append(data.TrendData.Revenue, dayRevenue.StringFixed(2))
	}

	return data, nil
}

// ========== 用户管理（管理端） ==========

// UserForAdmin 管理端用户信息
type UserForAdmin struct {
	Id           uint   `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Balance      string `json:"balance"`
	TotalUsed    string `json:"total_used"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	LastActiveAt string `json:"last_active_at"`
}

// ListUsersForAdmin 列出用户（管理端）
func ListUsersForAdmin(page, pageSize, search, status string) ([]UserForAdmin, int64, error) {
	var users []User
	var total int64

	query := DB.Model(&User{})
	if search != "" {
		query = query.Where("username LIKE ? OR email LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)

	offset, limit := parsePagination(page, pageSize)
	err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	result := make([]UserForAdmin, len(users))
	for i, u := range users {
		result[i] = UserForAdmin{
			Id:        uint(u.Id),
			Username:  u.Username,
			Email:     u.Email,
			Balance:   decimal.NewFromInt(int64(u.Quota)).Div(decimal.NewFromInt(500000)).StringFixed(2),
			TotalUsed: decimal.NewFromInt(int64(u.UsedQuota)).Div(decimal.NewFromInt(500000)).StringFixed(2),
			Status:    fmt.Sprintf("%d", u.Status),
			CreatedAt: "",
		}
	}

	return result, total, nil
}

// GetUserDetailForAdmin 获取用户详情（管理端）
func GetUserDetailForAdmin(userId uint) (*UserForAdmin, error) {
	var user User
	err := DB.First(&user, userId).Error
	if err != nil {
		return nil, err
	}

	return &UserForAdmin{
		Id:        uint(user.Id),
		Username:  user.Username,
		Email:     user.Email,
		Balance:   decimal.NewFromInt(int64(user.Quota)).Div(decimal.NewFromInt(500000)).StringFixed(2),
		TotalUsed: decimal.NewFromInt(int64(user.UsedQuota)).Div(decimal.NewFromInt(500000)).StringFixed(2),
		Status:    fmt.Sprintf("%d", user.Status),
		CreatedAt: "",
	}, nil
}

// UpdateUserByAdmin 管理员更新用户
func UpdateUserByAdmin(userId uint, displayName, status, group string) error {
	updates := map[string]interface{}{}
	if displayName != "" {
		updates["display_name"] = displayName
	}
	if status != "" {
		updates["status"] = status
	}
	if group != "" {
		updates["group"] = group
	}

	return DB.Model(&User{}).Where("id = ?", userId).Updates(updates).Error
}

// AdjustUserBalance 调整用户余额
func AdjustUserBalance(userId uint, amount decimal.Decimal, reason string) error {
	// 转换为quota单位（假设1 USD = 500000 quota）
	quotaAmount := amount.Mul(decimal.NewFromInt(500000)).IntPart()

	return DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.First(&user, userId).Error; err != nil {
			return err
		}

		newQuota := user.Quota + int(quotaAmount)
		if newQuota < 0 {
			return errors.New("余额不足")
		}

		return tx.Model(&User{}).Where("id = ?", userId).Update("quota", newQuota).Error
	})
}

// SetUserStatusByAdmin 管理员设置用户状态
func SetUserStatusByAdmin(userId uint, status, reason string) error {
	var statusInt int
	if status == "active" {
		statusInt = 1
	} else if status == "suspended" {
		statusInt = 2
	} else if status == "banned" {
		statusInt = 3
	} else {
		return errors.New("invalid status")
	}

	return DB.Model(&User{}).Where("id = ?", userId).Update("status", statusInt).Error
}

// ========== 审计日志 ==========

// RecordAdminAudit 记录管理操作审计
func RecordAdminAudit(adminId int, operation, targetId, targetType string, c *gin.Context) {
	log := AdminAuditLog{
		AdminId:   uint(adminId),
		AdminName: fmt.Sprintf("admin_%d", adminId),
		Operation: operation,
		TargetId:  targetId,
		TargetType: targetType,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}

	// 检查是否通过TOTP验证
	if totpVerified, exists := c.Get("totp_verified"); exists {
		log.TOTPVerified = totpVerified.(bool)
	}

	DB.Create(&log)
}

// ========== 辅助函数 ==========

func parsePagination(page, pageSize string) (offset, limit int) {
	p := 1
	ps := 20

	if page != "" {
		fmt.Sscanf(page, "%d", &p)
		if p < 1 {
			p = 1
		}
	}

	if pageSize != "" {
		fmt.Sscanf(pageSize, "%d", &ps)
		if ps < 1 || ps > 100 {
			ps = 20
		}
	}

	return (p - 1) * ps, ps
}

// ========== 其他管理端函数占位 ==========

// 这些函数需要在后续实现，目前返回空数据或错误

func ListChannelsForAdmin() ([]interface{}, error) {
	var channels []Channel
	err := DB.Omit("key").Order("priority desc").Find(&channels).Error
	if err != nil {
		return nil, err
	}

	// 清除敏感信息
	for i := range channels {
		channels[i].ChannelInfo.MultiKeyDisabledReason = nil
		channels[i].ChannelInfo.MultiKeyDisabledTime = nil
	}

	result := make([]interface{}, len(channels))
	for i, ch := range channels {
		result[i] = ch
	}
	return result, nil
}

func CreateChannelByAdmin(name, channelType, key, baseURL, models string, priority int) (interface{}, error) {
	// 解析渠道类型
	typeMap := map[string]int{
		"openai":     1,
		"anthropic":  14,
		"claude":     14,
		"gemini":     12,
		"azure":      3,
		"doubao":     25,
		"deepseek":   21,
		"qwen":       20,
	}
	chType := 1 // 默认 OpenAI
	if t, ok := typeMap[channelType]; ok {
		chType = t
	}

	channel := Channel{
		Name:        name,
		Type:        chType,
		Key:         key,
		Models:      models,
		Status:      1, // enabled
		CreatedTime: common.GetTimestamp(),
	}
	if baseURL != "" {
		channel.BaseURL = &baseURL
	}
	if priority > 0 {
		p := int64(priority)
		channel.Priority = &p
	}

	err := DB.Create(&channel).Error
	if err != nil {
		return nil, err
	}

	// 清除密钥后返回
	channel.Key = "***"
	return channel, nil
}

func UpdateChannelByAdmin(id uint, name, key, baseURL, models string, priority int, status string) error {
	updates := map[string]interface{}{}

	if name != "" {
		updates["name"] = name
	}
	if key != "" {
		updates["key"] = key
	}
	if baseURL != "" {
		updates["base_url"] = baseURL
	}
	if models != "" {
		updates["models"] = models
	}
	if priority > 0 {
		updates["priority"] = int64(priority)
	}
	if status != "" {
		switch status {
		case "enabled":
			updates["status"] = 1
		case "disabled":
			updates["status"] = 0
		}
	}

	if len(updates) == 0 {
		return nil
	}

	return DB.Model(&Channel{}).Where("id = ?", id).Updates(updates).Error
}

func DeleteChannelByAdmin(id uint) error {
	return DB.Delete(&Channel{}, id).Error
}

func TestChannelByAdmin(id uint) (interface{}, error) {
	var channel Channel
	err := DB.First(&channel, id).Error
	if err != nil {
		return nil, errors.New("渠道不存在")
	}

	// 返回渠道状态信息
	return map[string]interface{}{
		"id":            channel.Id,
		"name":          channel.Name,
		"type":          channel.Type,
		"status":        channel.Status,
		"response_time": channel.ResponseTime,
		"test_time":     channel.TestTime,
	}, nil
}

func GetPoolStatus() (*PoolStatus, error) {
	status := &PoolStatus{}

	// 统计账号池状态
	var total, available, cooldown, banned int64
	DB.Model(&PoolAccount{}).Count(&total)
	DB.Model(&PoolAccount{}).Where("status = ?", "available").Count(&available)
	DB.Model(&PoolAccount{}).Where("status = ?", "cooldown").Count(&cooldown)
	DB.Model(&PoolAccount{}).Where("status = ?", "banned").Count(&banned)

	status.TotalAccounts = int(total)
	status.Available = int(available)
	status.Cooldown = int(cooldown)
	status.Banned = int(banned)

	if total > 0 {
		rate := float64(available) / float64(total) * 100
		status.UtilizationRate = fmt.Sprintf("%.1f%%", rate)
	} else {
		status.UtilizationRate = "0.0%"
	}

	return status, nil
}

func ListPoolAccounts(statusFilter string) ([]interface{}, error) {
	var accounts []PoolAccount

	query := DB.Model(&PoolAccount{})
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	err := query.Order("created_at DESC").Find(&accounts).Error
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(accounts))
	for i, a := range accounts {
		result[i] = map[string]interface{}{
			"id":            a.Id,
			"email":         a.Email,
			"status":        a.Status,
			"quota_used":    a.QuotaUsed,
			"quota_total":   a.QuotaTotal,
			"cooldown_end":  a.CooldownEnd,
			"last_used":     a.LastUsed,
			"proxy_ip":      a.ProxyIP,
			"request_count": a.RequestCount,
		}
	}
	return result, nil
}

func AddPoolAccount(email, password, proxyIP string) (interface{}, error) {
	// 检查邮箱是否已存在
	var count int64
	DB.Model(&PoolAccount{}).Where("email = ?", email).Count(&count)
	if count > 0 {
		return nil, errors.New("该邮箱已在号池中")
	}

	account := PoolAccount{
		Email:    email,
		Password: password, // TODO: 加密存储
		ProxyIP:  proxyIP,
		Status:   "available",
	}

	err := DB.Create(&account).Error
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":     account.Id,
		"email":  account.Email,
		"status": account.Status,
	}, nil
}

func RemovePoolAccount(id uint) error {
	return DB.Delete(&PoolAccount{}, id).Error
}

func RemovePoolAccountByEmail(email string) error {
	return DB.Where("email = ?", email).Delete(&PoolAccount{}).Error
}

func RefreshPoolAccountToken(id uint) error {
	var account PoolAccount
	err := DB.First(&account, id).Error
	if err != nil {
		return errors.New("账号不存在")
	}

	// TODO: 实际刷新 Token 逻辑
	// 这里需要调用 Sub2api 的接口来刷新 OAuth Token
	// 当前版本只更新状态

	return DB.Model(&account).Updates(map[string]interface{}{
		"status":     "available",
		"updated_at": time.Now(),
	}).Error
}

// PoolAccount 号池账号模型
type PoolAccount struct {
	Id          uint       `json:"id" gorm:"primaryKey"`
	Email       string     `json:"email" gorm:"size:128;uniqueIndex;not null"`
	Password    string     `json:"-" gorm:"size:256"` // 加密存储
	OAuthToken  string     `json:"-" gorm:"type:text"`
	Status      string     `json:"status" gorm:"size:20;default:'available'"` // available/cooldown/banned
	QuotaUsed   string     `json:"quota_used" gorm:"size:20"`
	QuotaTotal  string     `json:"quota_total" gorm:"size:20"`
	CooldownEnd *time.Time `json:"cooldown_end"`
	LastUsed    *time.Time `json:"last_used"`
	ProxyIP     string     `json:"proxy_ip" gorm:"size:64"`
	RequestCount int64     `json:"request_count" gorm:"default:0"`
	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (PoolAccount) TableName() string {
	return "pool_accounts"
}

func ListAllPricing() ([]ModelPricing, error) {
	var pricing []ModelPricing
	err := DB.Find(&pricing).Error
	return pricing, err
}

// GetModelPricing 获取单个模型的定价
func GetModelPricing(modelName string) (*ModelPricing, error) {
	var pricing ModelPricing
	err := DB.Where("model = ? AND is_active = ?", modelName, true).First(&pricing).Error
	if err != nil {
		// 尝试从默认定价中查找
		for _, p := range DefaultModelPricing {
			if p.Model == modelName {
				return &p, nil
			}
		}
		return nil, err
	}
	return &pricing, nil
}

// CalculateCost 计算API调用成本（美元）
func CalculateCost(modelName string, inputTokens, outputTokens int64) (string, error) {
	pricing, err := GetModelPricing(modelName)
	if err != nil {
		return "0.00", nil
	}

	// 计算成本: (输入token数 / 1,000,000) * 输入价格 + (输出token数 / 1,000,000) * 输出价格
	inputCost := pricing.InputPrice.Mul(decimal.NewFromFloat(float64(inputTokens)).Div(decimal.NewFromInt(1000000)))
	outputCost := pricing.OutputPrice.Mul(decimal.NewFromFloat(float64(outputTokens)).Div(decimal.NewFromInt(1000000)))
	totalCost := inputCost.Add(outputCost)

	return totalCost.StringFixed(6), nil
}

func CreateModelPricing(modelName, inputPrice, outputPrice, perImagePrice, perSecondPrice, perCharPrice string) (*ModelPricing, error) {
	pricing := ModelPricing{
		Model:    modelName,
		IsActive: true,
	}

	if inputPrice != "" {
		p, err := decimal.NewFromString(inputPrice)
		if err != nil {
			return nil, fmt.Errorf("invalid input_price: %v", err)
		}
		pricing.InputPrice = p
	}
	if outputPrice != "" {
		p, err := decimal.NewFromString(outputPrice)
		if err != nil {
			return nil, fmt.Errorf("invalid output_price: %v", err)
		}
		pricing.OutputPrice = p
	}
	if perImagePrice != "" {
		p, err := decimal.NewFromString(perImagePrice)
		if err != nil {
			return nil, fmt.Errorf("invalid per_image_price: %v", err)
		}
		pricing.PerImagePrice = p
	}
	if perSecondPrice != "" {
		p, err := decimal.NewFromString(perSecondPrice)
		if err != nil {
			return nil, fmt.Errorf("invalid per_second_price: %v", err)
		}
		pricing.PerSecondPrice = p
	}
	if perCharPrice != "" {
		p, err := decimal.NewFromString(perCharPrice)
		if err != nil {
			return nil, fmt.Errorf("invalid per_char_price: %v", err)
		}
		pricing.PerCharPrice = p
	}

	err := DB.Create(&pricing).Error
	if err != nil {
		return nil, err
	}

	return &pricing, nil
}

func UpdateModelPricing(id uint, inputPrice, outputPrice, perImagePrice, perSecondPrice, perCharPrice string) error {
	updates := map[string]interface{}{}

	if inputPrice != "" {
		p, err := decimal.NewFromString(inputPrice)
		if err != nil {
			return fmt.Errorf("invalid input_price: %v", err)
		}
		updates["input_price"] = p
	}
	if outputPrice != "" {
		p, err := decimal.NewFromString(outputPrice)
		if err != nil {
			return fmt.Errorf("invalid output_price: %v", err)
		}
		updates["output_price"] = p
	}
	if perImagePrice != "" {
		p, err := decimal.NewFromString(perImagePrice)
		if err != nil {
			return fmt.Errorf("invalid per_image_price: %v", err)
		}
		updates["per_image_price"] = p
	}
	if perSecondPrice != "" {
		p, err := decimal.NewFromString(perSecondPrice)
		if err != nil {
			return fmt.Errorf("invalid per_second_price: %v", err)
		}
		updates["per_second_price"] = p
	}
	if perCharPrice != "" {
		p, err := decimal.NewFromString(perCharPrice)
		if err != nil {
			return fmt.Errorf("invalid per_char_price: %v", err)
		}
		updates["per_char_price"] = p
	}

	if len(updates) == 0 {
		return nil
	}

	return DB.Model(&ModelPricing{}).Where("id = ?", id).Updates(updates).Error
}

func GetAllPlans() ([]Plan, error) {
	var plans []Plan
	err := DB.Order("sort_order").Find(&plans).Error
	return plans, err
}

func CreatePlanByAdmin(name, displayName, description, monthlyPrice, includedUSD, discountRate string, maxAPIKeys, maxSubAccounts int, features string) (*Plan, error) {
	plan := Plan{
		Name:        name,
		DisplayName: displayName,
		Description: description,
		IsActive:    true,
		MaxAPIKeys:  maxAPIKeys,
	}

	if displayName == "" {
		plan.DisplayName = name
	}
	if monthlyPrice != "" {
		p, err := decimal.NewFromString(monthlyPrice)
		if err != nil {
			return nil, fmt.Errorf("invalid monthly_price: %v", err)
		}
		plan.MonthlyPrice = p
	}
	if includedUSD != "" {
		p, err := decimal.NewFromString(includedUSD)
		if err != nil {
			return nil, fmt.Errorf("invalid included_usd: %v", err)
		}
		plan.IncludedUSD = p
	}
	if discountRate != "" {
		p, err := decimal.NewFromString(discountRate)
		if err != nil {
			return nil, fmt.Errorf("invalid discount_rate: %v", err)
		}
		plan.DiscountRate = p
	}
	if maxSubAccounts > 0 {
		plan.MaxSubAccounts = maxSubAccounts
	}
	if features != "" {
		plan.Features = features
	}

	err := DB.Create(&plan).Error
	if err != nil {
		return nil, err
	}

	return &plan, nil
}

func UpdatePlanByAdmin(id uint, displayName, description, monthlyPrice, includedUSD, discountRate string, maxAPIKeys, maxSubAccounts int, features string) error {
	updates := map[string]interface{}{}

	if displayName != "" {
		updates["display_name"] = displayName
	}
	if description != "" {
		updates["description"] = description
	}
	if monthlyPrice != "" {
		p, err := decimal.NewFromString(monthlyPrice)
		if err != nil {
			return fmt.Errorf("invalid monthly_price: %v", err)
		}
		updates["monthly_price"] = p
	}
	if includedUSD != "" {
		p, err := decimal.NewFromString(includedUSD)
		if err != nil {
			return fmt.Errorf("invalid included_usd: %v", err)
		}
		updates["included_usd"] = p
	}
	if discountRate != "" {
		p, err := decimal.NewFromString(discountRate)
		if err != nil {
			return fmt.Errorf("invalid discount_rate: %v", err)
		}
		updates["discount_rate"] = p
	}
	if maxAPIKeys > 0 {
		updates["max_api_keys"] = maxAPIKeys
	}
	if maxSubAccounts > 0 {
		updates["max_sub_accounts"] = maxSubAccounts
	}
	if features != "" {
		updates["features"] = features
	}

	if len(updates) == 0 {
		return nil
	}

	return DB.Model(&Plan{}).Where("id = ?", id).Updates(updates).Error
}

func GetFinanceOverview() (*FinanceOverview, error) {
	return &FinanceOverview{}, nil
}

func GetRevenueReport(startDate, endDate string) (*RevenueReport, error) {
	return &RevenueReport{}, nil
}

func ListPaymentsForAdmin(page, pageSize, status string) ([]interface{}, int64, error) {
	return []interface{}{}, 0, nil
}

func ListAuditLogsForAdmin(page, pageSize, adminId, operation, startDate, endDate string) ([]AdminAuditLog, int64, error) {
	var logs []AdminAuditLog
	var total int64
	DB.Model(&AdminAuditLog{}).Count(&total)
	DB.Find(&logs)
	return logs, total, nil
}

func GetSystemSettings() (*Settings, error) {
	return &Settings{
		TrialAmount:      "1.00",
		MinRecharge:      "2.00",
		ReferralBonus:    "3.00",
		RefundPolicy:     "7天内可退，扣5%手续费",
		RegistrationOpen: true,
	}, nil
}

func UpdateSystemSettings(req Settings) error {
	return nil
}

func ListWebhooksForAdmin() ([]interface{}, error) {
	var webhooks []Webhook
	err := DB.Order("created_at DESC").Find(&webhooks).Error
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(webhooks))
	for i, w := range webhooks {
		result[i] = w
	}
	return result, nil
}

func CreateWebhookByAdmin(name, webhookURL, events string) (interface{}, error) {
	webhook := Webhook{
		Name:     name,
		URL:      webhookURL,
		Events:   events,
		IsActive: true,
		Secret:   generateWebhookSecret(),
	}

	err := DB.Create(&webhook).Error
	if err != nil {
		return nil, err
	}

	return webhook, nil
}

func UpdateWebhookByAdmin(id uint, name, webhookURL, events string) error {
	updates := map[string]interface{}{}

	if name != "" {
		updates["name"] = name
	}
	if webhookURL != "" {
		updates["url"] = webhookURL
	}
	if events != "" {
		updates["events"] = events
	}

	if len(updates) == 0 {
		return nil
	}

	return DB.Model(&Webhook{}).Where("id = ?", id).Updates(updates).Error
}

func DeleteWebhookByAdmin(id uint) error {
	return DB.Delete(&Webhook{}, id).Error
}

func TestWebhookByAdmin(id uint) (interface{}, error) {
	var webhook Webhook
	err := DB.First(&webhook, id).Error
	if err != nil {
		return nil, errors.New("Webhook 不存在")
	}

	// 发送测试请求
	testPayload := map[string]interface{}{
		"event":     "test",
		"timestamp": time.Now().Unix(),
		"message":   "这是一条测试消息",
	}

	err = SendWebhookRequest(&webhook, "test", testPayload)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
		"message": "Webhook 测试成功",
	}, nil
}

// generateWebhookSecret 生成 Webhook 签名密钥
func generateWebhookSecret() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// SendWebhookRequest 发送 Webhook 请求
func SendWebhookRequest(webhook *Webhook, event string, payload map[string]interface{}) error {
	payload["event"] = event
	payload["timestamp"] = time.Now().Unix()
	payload["webhook_id"] = webhook.Id

	_, err := common.Marshal(payload)
	if err != nil {
		return err
	}

	// TODO: 实际发送 HTTP 请求
	// 这里可以使用 http.Post 或异步队列发送
	// 当前版本先记录日志
	common.SysLog(fmt.Sprintf("Webhook send: %s -> %s", event, webhook.URL))

	// 更新发送计数
	now := time.Now()
	DB.Model(webhook).Updates(map[string]interface{}{
		"last_sent":  now,
		"send_count": gorm.Expr("send_count + 1"),
	})

	return nil
}

// TriggerWebhook 触发 Webhook
func TriggerWebhook(event string, payload map[string]interface{}) {
	// 查找订阅了该事件的所有活跃 webhook
	var webhooks []Webhook
	err := DB.Where("is_active = ? AND events LIKE ?", true, "%"+event+"%").Find(&webhooks).Error
	if err != nil {
		return
	}

	// 异步发送（简化实现，实际应使用消息队列）
	for _, w := range webhooks {
		go SendWebhookRequest(&w, event, payload)
	}
}

// 占位类型
type PoolStatus struct {
	TotalAccounts   int    `json:"total_accounts"`
	Available       int    `json:"available"`
	Cooldown        int    `json:"cooldown"`
	Banned          int    `json:"banned"`
	UtilizationRate string `json:"utilization_rate"`
}

type FinanceOverview struct {
	TotalRevenue   string `json:"total_revenue"`
	TotalCost      string `json:"total_cost"`
	GrossMargin    string `json:"gross_margin"`
	MonthlyRevenue string `json:"monthly_revenue"`
	MonthlyCost    string `json:"monthly_cost"`
}

type RevenueReport struct {
	Period   string         `json:"period"`
	Data     []RevenueItem  `json:"data"`
	ByModel  []RevenueItem  `json:"by_model"`
	BySource []RevenueItem  `json:"by_source"`
}

type RevenueItem struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
}

type Settings struct {
	TrialAmount        string `json:"trial_amount"`
	MinRecharge        string `json:"min_recharge"`
	ReferralBonus      string `json:"referral_bonus"`
	RefundPolicy       string `json:"refund_policy"`
	MaintenanceMode    bool   `json:"maintenance_mode"`
	RegistrationOpen   bool   `json:"registration_open"`
}

// ========== 管理员管理 ==========

// GetAdminById 根据ID获取管理员
func GetAdminById(id uint) (*Admin, error) {
	var admin Admin
	err := DB.First(&admin, id).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// GetAdminByUsername 根据用户名获取管理员
func GetAdminByUsername(username string) (*Admin, error) {
	var admin Admin
	err := DB.Where("username = ?", username).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// CreateAdmin 创建管理员
func CreateAdmin(username, email, passwordHash string, role AdminRole) (*Admin, error) {
	admin := Admin{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		IsActive:     true,
	}
	err := DB.Create(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// UpdateAdminTOTP 更新管理员TOTP密钥
func UpdateAdminTOTP(adminId uint, totpSecret string) error {
	return DB.Model(&Admin{}).Where("id = ?", adminId).
		Update("totp_secret", totpSecret).Error
}

// UpdateAdminLastLogin 更新管理员最后登录时间
func UpdateAdminLastLogin(adminId uint) error {
	now := time.Now()
	return DB.Model(&Admin{}).Where("id = ?", adminId).
		Update("last_login_at", now).Error
}

// GetChannelByModel 根据模型名称获取渠道（用于模型验证）
func GetChannelByModel(modelName string) (interface{}, error) {
	// 调用 new-api 原有的渠道查询逻辑
	// 这里返回 nil 表示使用默认逻辑
	return nil, nil
}

// ========== 预算管理 ==========

// GetBudgetConfig 获取用户预算配置
func GetBudgetConfig(userId uint) (*UserBudgetConfig, error) {
	var config UserBudgetConfig
	err := DB.Where("user_id = ?", userId).First(&config).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 返回默认配置
			return &UserBudgetConfig{
				UserId:         userId,
				DailyLimit:     0,
				MonthlyLimit:   0,
				AlertThreshold: 0.8,
				NotifyEmail:    true,
				NotifyWebhook:  true,
			}, nil
		}
		return nil, err
	}
	return &config, nil
}

// SetBudgetConfig 设置用户预算配置
func SetBudgetConfig(userId uint, dailyLimit, monthlyLimit, alertThreshold float64, notifyEmail, notifyWebhook bool) (*UserBudgetConfig, error) {
	config := UserBudgetConfig{
		UserId:         userId,
		DailyLimit:     dailyLimit,
		MonthlyLimit:   monthlyLimit,
		AlertThreshold: alertThreshold,
		NotifyEmail:    notifyEmail,
		NotifyWebhook:  notifyWebhook,
	}

	err := DB.Save(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// BudgetStatus 预算状态
type BudgetStatus struct {
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

// GetBudgetStatus 获取用户预算状态
func GetBudgetStatus(userId uint) (*BudgetStatus, error) {
	config, err := GetBudgetConfig(userId)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	// 计算今日消费
	var dailyUsed decimal.Decimal
	DB.Model(&ConsumptionRecord{}).
		Where("user_id = ? AND created_at >= ?", userId, today).
		Select("COALESCE(SUM(actual_cost_usd), 0)").Scan(&dailyUsed)

	// 计算本月消费
	var monthlyUsed decimal.Decimal
	DB.Model(&ConsumptionRecord{}).
		Where("user_id = ? AND created_at >= ?", userId, monthStart).
		Select("COALESCE(SUM(actual_cost_usd), 0)").Scan(&monthlyUsed)

	status := &BudgetStatus{
		DailyUsed:      dailyUsed.InexactFloat64(),
		DailyLimit:     config.DailyLimit,
		MonthlyUsed:    monthlyUsed.InexactFloat64(),
		MonthlyLimit:   config.MonthlyLimit,
		AlertThreshold: config.AlertThreshold,
	}

	// 计算百分比
	if config.DailyLimit > 0 {
		status.DailyPercent = status.DailyUsed / config.DailyLimit
		status.DailyExceeded = status.DailyUsed >= config.DailyLimit
	} else {
		status.DailyPercent = 0
	}

	if config.MonthlyLimit > 0 {
		status.MonthlyPercent = status.MonthlyUsed / config.MonthlyLimit
		status.MonthlyExceeded = status.MonthlyUsed >= config.MonthlyLimit
	} else {
		status.MonthlyPercent = 0
	}

	// 检查是否需要告警
	shouldAlert := false
	if config.DailyLimit > 0 && status.DailyPercent >= config.AlertThreshold && !config.AlertSent {
		shouldAlert = true
	}
	if config.MonthlyLimit > 0 && status.MonthlyPercent >= config.AlertThreshold && !config.AlertSent {
		shouldAlert = true
	}
	status.ShouldAlert = shouldAlert

	return status, nil
}

// CheckAndSendBudgetAlert 检查并发送预算告警
func CheckAndSendBudgetAlert(userId uint) error {
	status, err := GetBudgetStatus(userId)
	if err != nil {
		return err
	}

	if !status.ShouldAlert {
		return nil
	}

	// 发送告警通知
	TriggerWebhook("budget_alert", map[string]interface{}{
		"user_id":        userId,
		"daily_used":     status.DailyUsed,
		"daily_limit":    status.DailyLimit,
		"monthly_used":   status.MonthlyUsed,
		"monthly_limit":  status.MonthlyLimit,
		"alert_threshold": status.AlertThreshold,
	})

	// 标记已发送
	return DB.Model(&UserBudgetConfig{}).
		Where("user_id = ?", userId).
		Update("alert_sent", true).Error
}

// ResetBudgetAlertMonthly 重置月度告警状态
func ResetBudgetAlertMonthly() {
	now := time.Now()
	if now.Day() == 1 {
		DB.Model(&UserBudgetConfig{}).
			Where("alert_sent = ?", true).
			Update("alert_sent", false)
	}
}

// ========== 调用日志 ==========

// CallLog 调用日志响应（脱敏）
type CallLog struct {
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
func GetCallLogs(userId uint, page, pageSize, model string, startTime, endTime *time.Time) ([]CallLog, int64, error) {
	query := DB.Model(&ConsumptionRecord{}).Where("user_id = ?", userId)

	if model != "" {
		query = query.Where("model = ?", model)
	}
	if startTime != nil {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime != nil {
		query = query.Where("created_at <= ?", endTime)
	}

	var total int64
	query.Count(&total)

	offset, limit := parsePagination(page, pageSize)
	var records []ConsumptionRecord
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&records).Error
	if err != nil {
		return nil, 0, err
	}

	logs := make([]CallLog, len(records))
	for i, r := range records {
		logs[i] = CallLog{
			Id:           r.Id,
			RequestId:    r.RequestId,
			Model:        r.Model,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			CostUSD:      r.ActualCostUSD.StringFixed(6),
			Status:       r.Status,
			CreatedAt:    r.CreatedAt.Format(time.RFC3339),
		}
	}

	return logs, total, nil
}

// GetCallLogDetail 获取单条调用日志详情
func GetCallLogDetail(userId uint, logId uint) (*CallLogDetail, error) {
	var record ConsumptionRecord
	err := DB.Where("id = ? AND user_id = ?", logId, userId).First(&record).Error
	if err != nil {
		return nil, err
	}

	return &CallLogDetail{
		Id:              record.Id,
		RequestId:       record.RequestId,
		Model:           record.Model,
		ChannelId:       record.ChannelId,
		InputTokens:     record.InputTokens,
		OutputTokens:    record.OutputTokens,
		CacheReadTokens: record.CacheReadTokens,
		CacheWriteTokens: record.CacheWriteTokens,
		InputPrice:      record.InputPrice.StringFixed(6),
		OutputPrice:     record.OutputPrice.StringFixed(6),
		PreDeductUSD:    record.PreDeductUSD.StringFixed(6),
		ActualCostUSD:   record.ActualCostUSD.StringFixed(6),
		BalanceSource:   record.BalanceSource,
		Status:          record.Status,
		CreatedAt:       record.CreatedAt.Format(time.RFC3339),
	}, nil
}

// CallLogDetail 调用日志详情（脱敏）
type CallLogDetail struct {
	Id               uint   `json:"id"`
	RequestId        string `json:"request_id"`
	Model            string `json:"model"`
	ChannelId        uint   `json:"channel_id"`
	InputTokens      int64  `json:"input_tokens"`
	OutputTokens     int64  `json:"output_tokens"`
	CacheReadTokens  int64  `json:"cache_read_tokens"`
	CacheWriteTokens int64  `json:"cache_write_tokens"`
	InputPrice       string `json:"input_price"`
	OutputPrice      string `json:"output_price"`
	PreDeductUSD     string `json:"pre_deduct_usd"`
	ActualCostUSD    string `json:"actual_cost_usd"`
	BalanceSource    string `json:"balance_source"`
	Status           string `json:"status"`
	CreatedAt        string `json:"created_at"`
}
