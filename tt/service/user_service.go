// Package service 提供TT业务服务层
package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	ttmodel "github.com/QuantumNous/new-api/model"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ========== 用户服务 ==========

// UserService 用户服务
type UserService struct {
	db *gorm.DB
}

// NewUserService 创建用户服务
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// GrantTrialBalance 发放注册赠送余额
func (s *UserService) GrantTrialBalance(userId int) error {
	return ttmodel.GrantTrialBalance(userId)
}

// GetUserBalance 获取用户余额信息
func (s *UserService) GetUserBalance(userId int) (*BalanceInfo, error) {
	ext, err := ttmodel.GetUserExtension(userId)
	if err != nil {
		return nil, err
	}

	// 计算总可用余额（付费余额 + 赠送余额 - 已用赠送）
	trialAvailable := ext.TrialBalance.Sub(ext.TrialUsed)
	if trialAvailable.IsNegative() {
		trialAvailable = decimal.Zero
	}

	return &BalanceInfo{
		UserId:         userId,
		TrialBalance:   ext.TrialBalance,
		TrialUsed:      ext.TrialUsed,
		TrialAvailable: trialAvailable,
		TrialGrantedAt: ext.TrialGrantedAt,
	}, nil
}

// BalanceInfo 余额信息
type BalanceInfo struct {
	UserId         int            `json:"user_id"`
	TrialBalance   decimal.Decimal `json:"trial_balance"`
	TrialUsed      decimal.Decimal `json:"trial_used"`
	TrialAvailable decimal.Decimal `json:"trial_available"`
	TrialGrantedAt *time.Time     `json:"trial_granted_at,omitempty"`
}

// ConsumeBalance 消费余额
// 优先级：赠送余额 -> 付费余额
func (s *UserService) ConsumeBalance(userId int, amount decimal.Decimal, source string) error {
	ext, err := ttmodel.GetUserExtension(userId)
	if err != nil {
		return err
	}

	// 优先使用赠送余额
	if ext.TrialBalance.Sub(ext.TrialUsed).Cmp(amount) >= 0 {
		// 赠送余额足够
		ext.TrialUsed = ext.TrialUsed.Add(amount)
		return s.db.Save(ext).Error
	}

	// 赠送余额不足，先用完赠送余额，再用付费余额
	trialAvailable := ext.TrialBalance.Sub(ext.TrialUsed)
	remaining := amount.Sub(trialAvailable)

	ext.TrialUsed = ext.TrialBalance

	// 扣除付费余额（通过修改用户quota）
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(ext).Error; err != nil {
			return err
		}

		// 扣除付费余额（转换为quota单位）
		quotaAmount := remaining.Mul(decimal.NewFromInt(500000)).IntPart()
		return tx.Exec("UPDATE users SET quota = quota - ? WHERE id = ? AND quota >= ?",
			quotaAmount, userId, quotaAmount).Error
	})
}

// ========== 用量服务 ==========

// UsageService 用量服务
type UsageService struct {
	db *gorm.DB
}

// NewUsageService 创建用量服务
func NewUsageService(db *gorm.DB) *UsageService {
	return &UsageService{db: db}
}

// RecordUsage 记录用量
func (s *UsageService) RecordUsage(userId int, req *UsageRecord) error {
	record := ttmodel.ConsumptionRecord{
		UserId:        uint(userId),
		RequestId:     req.RequestId,
		Model:         req.Model,
		ChannelId:     req.ChannelId,
		InputTokens:   req.InputTokens,
		OutputTokens:  req.OutputTokens,
		InputPrice:    req.InputPrice,
		OutputPrice:   req.OutputPrice,
		PreDeductUSD:  req.PreDeductUSD,
		ActualCostUSD: req.ActualCostUSD,
		BalanceSource: req.BalanceSource,
		Status:        "completed",
	}

	return s.db.Create(&record).Error
}

// UsageRecord 用量记录
type UsageRecord struct {
	RequestId     string          `json:"request_id"`
	Model         string          `json:"model"`
	ChannelId     uint            `json:"channel_id"`
	InputTokens   int64           `json:"input_tokens"`
	OutputTokens  int64           `json:"output_tokens"`
	InputPrice    decimal.Decimal `json:"input_price"`
	OutputPrice   decimal.Decimal `json:"output_price"`
	PreDeductUSD  decimal.Decimal `json:"pre_deduct_usd"`
	ActualCostUSD decimal.Decimal `json:"actual_cost_usd"`
	BalanceSource string          `json:"balance_source"`
}

// GetUsageStats 获取用量统计
func (s *UsageService) GetUsageStats(userId int, startTime time.Time) (*ttmodel.UsageStats, error) {
	return ttmodel.GetUserUsage(userId, startTime)
}

// ========== 邀请裂变服务 ==========

// ReferralService 邀请裂变服务
type ReferralService struct {
	db *gorm.DB
}

// NewReferralService 创建邀请裂变服务
func NewReferralService(db *gorm.DB) *ReferralService {
	return &ReferralService{db: db}
}

// GenerateInviteCode 生成邀请码
func (s *ReferralService) GenerateInviteCode(userId int) (string, error) {
	// 使用用户ID生成唯一邀请码
	code := fmt.Sprintf("TK%d%s", userId, randomString(6))
	return code, nil
}

// ApplyInviteCode 使用邀请码
func (s *ReferralService) ApplyInviteCode(userId int, code string, ip string, deviceFingerprint string) (*ttmodel.Referral, error) {
	// 查找邀请人
	var inviter ttmodel.User
	if err := s.db.Where("aff_code = ?", code).First(&inviter).Error; err != nil {
		return nil, errors.New("无效的邀请码")
	}

	return ttmodel.ApplyReferralCode(userId, code, ip)
}

// ProcessFirstCharge 处理首次充值（触发邀请奖励）
func (s *ReferralService) ProcessFirstCharge(userId int) error {
	// 查找待发放的邀请记录
	var referral ttmodel.Referral
	err := s.db.Where("invitee_id = ? AND status = ?", userId, "pending").First(&referral).Error
	if err != nil {
		return nil // 没有待发放的邀请记录
	}

	// 发放奖励
	config := ttmodel.DefaultReferralConfig
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 更新邀请记录状态
		now := time.Now()
		referral.Status = "granted"
		referral.GrantedAt = &now
		referral.GrantReason = "first_charge"

		if err := tx.Save(&referral).Error; err != nil {
			return err
		}

		// 给邀请人发放奖励
		inviterExt, err := ttmodel.GetUserExtension(int(referral.InviterId))
		if err != nil {
			return err
		}
		inviterExt.TrialBalance = inviterExt.TrialBalance.Add(referral.InviterBonus)
		if err := tx.Save(inviterExt).Error; err != nil {
			return err
		}

		// 给被邀请人发放奖励
		inviteeExt, err := ttmodel.GetUserExtension(int(referral.InviteeId))
		if err != nil {
			return err
		}
		inviteeExt.TrialBalance = inviteeExt.TrialBalance.Add(referral.InviteeBonus)
		return tx.Save(inviteeExt).Error
	})

	return err
}

// ========== 订阅服务 ==========

// SubscriptionService 订阅服务
type SubscriptionService struct {
	db *gorm.DB
}

// NewSubscriptionService 创建订阅服务
func NewSubscriptionService(db *gorm.DB) *SubscriptionService {
	return &SubscriptionService{db: db}
}

// Subscribe 订阅套餐
func (s *SubscriptionService) Subscribe(userId int, planId uint, billingCycle string) (*ttmodel.Subscription, error) {
	return ttmodel.CreateSubscription(userId, planId, billingCycle)
}

// CancelSubscription 取消订阅
func (s *SubscriptionService) CancelSubscription(userId int, reason string) error {
	return ttmodel.CancelUserSubscription(userId, reason)
}

// CheckSubscription 检查订阅状态
func (s *SubscriptionService) CheckSubscription(userId int) (*SubscriptionStatus, error) {
	info, err := ttmodel.GetUserSubscription(userId)
	if err != nil {
		return &SubscriptionStatus{HasSubscription: false}, nil
	}

	return &SubscriptionStatus{
		HasSubscription: info.HasSubscription,
		PlanName:        info.PlanName,
		Status:          info.Status,
		ExpiresAt:       info.ExpiresAt,
		UsedUSD:         info.UsedUSD,
		RemainingUSD:    info.RemainingUSD,
	}, nil
}

// SubscriptionStatus 订阅状态
type SubscriptionStatus struct {
	HasSubscription bool   `json:"has_subscription"`
	PlanName        string `json:"plan_name,omitempty"`
	Status          string `json:"status,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	UsedUSD         string `json:"used_usd,omitempty"`
	RemainingUSD    string `json:"remaining_usd,omitempty"`
}

// RenewSubscription 续费订阅
func (s *SubscriptionService) RenewSubscription(userId int) error {
	var sub ttmodel.Subscription
	err := s.db.Where("user_id = ? AND status IN ?", userId, []string{"active", "cancelled"}).
		Preload("Plan").First(&sub).Error
	if err != nil {
		return errors.New("no active subscription")
	}

	// 计算新的过期时间
	var newExpires time.Time
	if sub.BillingCycle == "yearly" {
		newExpires = sub.ExpiresAt.AddDate(1, 0, 0)
	} else {
		newExpires = sub.ExpiresAt.AddDate(0, 1, 0)
	}

	// 更新订阅
	updates := map[string]interface{}{
		"expires_at":    newExpires,
		"status":        "active",
		"remaining_usd": sub.Plan.IncludedUSD,
		"used_usd":      0,
	}

	return s.db.Model(&sub).Updates(updates).Error
}

// ========== 团队服务 ==========

// TeamService 团队服务
type TeamService struct {
	db *gorm.DB
}

// NewTeamService 创建团队服务
func NewTeamService(db *gorm.DB) *TeamService {
	return &TeamService{db: db}
}

// CreateTeam 创建团队
func (s *TeamService) CreateTeam(ownerId uint, name string, description string) (*ttmodel.Team, error) {
	return ttmodel.CreateTeam(ownerId, name, description, 0)
}

// AddMember 添加成员
func (s *TeamService) AddMember(teamId, userId uint, role string) error {
	return ttmodel.AddTeamMember(teamId, userId, role)
}

// RemoveMember 移除成员
func (s *TeamService) RemoveMember(teamId, userId uint) error {
	return ttmodel.RemoveTeamMember(teamId, userId)
}

// ========== Webhook服务 ==========

// WebhookService Webhook服务
type WebhookService struct {
	db *gorm.DB
}

// NewWebhookService 创建Webhook服务
func NewWebhookService(db *gorm.DB) *WebhookService {
	return &WebhookService{db: db}
}

// WebhookConfig Webhook配置
type WebhookConfig struct {
	Id      uint   `json:"id"`
	UserId  uint   `json:"user_id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Events  string `json:"events"` // comma separated
	Secret  string `json:"secret"`
	Active  bool   `json:"active"`
}

// SendWebhook 发送Webhook
func (s *WebhookService) SendWebhook(config *WebhookConfig, event string, payload map[string]interface{}) error {
	payload["event"] = event
	payload["timestamp"] = time.Now().Unix()
	payload["webhook_id"] = config.Id

	// 序列化payload
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// 发送HTTP POST请求
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("POST", config.URL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TokenKey-Webhook/1.0")

	// 如果有密钥，添加签名
	if config.Secret != "" {
		signature := hmacSHA256(body, []byte(config.Secret))
		req.Header.Set("X-Webhook-Signature", signature)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode >= 400 {
		return errors.New("webhook endpoint returned error: " + resp.Status)
	}

	return nil
}

// hmacSHA256 计算 HMAC-SHA256 签名
func hmacSHA256(data, key []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// ========== 预算告警服务 ==========

// BudgetService 预算告警服务
type BudgetService struct {
	db *gorm.DB
}

// NewBudgetService 创建预算告警服务
func NewBudgetService(db *gorm.DB) *BudgetService {
	return &BudgetService{db: db}
}

// BudgetConfig 预算配置
type BudgetConfig struct {
	UserId       uint    `json:"user_id"`
	DailyLimit   float64 `json:"daily_limit"`
	MonthlyLimit float64 `json:"monthly_limit"`
	AlertThreshold float64 `json:"alert_threshold"` // 告警阈值（0.8 = 80%）
}

// CheckBudget 检查预算
func (s *BudgetService) CheckBudget(userId uint, config *BudgetConfig) (*BudgetStatus, error) {
	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	// 计算今日消费
	var dailyUsed decimal.Decimal
	s.db.Model(&ttmodel.ConsumptionRecord{}).
		Where("user_id = ? AND created_at >= ?", userId, today).
		Select("COALESCE(SUM(actual_cost_usd), 0)").Scan(&dailyUsed)

	// 计算本月消费
	var monthlyUsed decimal.Decimal
	s.db.Model(&ttmodel.ConsumptionRecord{}).
		Where("user_id = ? AND created_at >= ?", userId, monthStart).
		Select("COALESCE(SUM(actual_cost_usd), 0)").Scan(&monthlyUsed)

	status := &BudgetStatus{
		DailyUsed:   dailyUsed.InexactFloat64(),
		MonthlyUsed: monthlyUsed.InexactFloat64(),
	}

	// 检查是否超限
	if config.DailyLimit > 0 && status.DailyUsed >= config.DailyLimit {
		status.DailyExceeded = true
	}
	if config.MonthlyLimit > 0 && status.MonthlyUsed >= config.MonthlyLimit {
		status.MonthlyExceeded = true
	}

	// 检查是否需要告警
	threshold := config.AlertThreshold
	if threshold == 0 {
		threshold = 0.8
	}

	if config.DailyLimit > 0 && status.DailyUsed/config.DailyLimit >= threshold {
		status.DailyAlert = true
	}
	if config.MonthlyLimit > 0 && status.MonthlyUsed/config.MonthlyLimit >= threshold {
		status.MonthlyAlert = true
	}

	return status, nil
}

// BudgetStatus 预算状态
type BudgetStatus struct {
	DailyUsed       float64 `json:"daily_used"`
	MonthlyUsed     float64 `json:"monthly_used"`
	DailyExceeded   bool    `json:"daily_exceeded"`
	MonthlyExceeded bool    `json:"monthly_exceeded"`
	DailyAlert      bool    `json:"daily_alert"`
	MonthlyAlert    bool    `json:"monthly_alert"`
}

// ========== 辅助函数 ==========

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
