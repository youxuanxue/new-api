// Package model 提供TT数据模型
// billing_enhanced.go - 计费增强模型：注册赠送、邀请裂变、月度套餐
package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// ========== 用户扩展字段 ==========
// 注意：由于new-api已有User模型，我们通过创建UserExtension表来扩展
// 或者可以通过ALTER TABLE添加字段（需要迁移脚本）

// UserExtension 用户扩展信息表
// 一对一关联到users表
type UserExtension struct {
	Id        uint            `json:"id" gorm:"primaryKey"`
	UserId    uint            `json:"user_id" gorm:"uniqueIndex;not null"`

	// 注册赠送
	TrialBalance    decimal.Decimal `json:"trial_balance" gorm:"type:decimal(10,6);default:0"`
	TrialUsed       decimal.Decimal `json:"trial_used" gorm:"type:decimal(10,6);default:0"`
	TrialGrantedAt  *time.Time      `json:"trial_granted_at"`

	// 邀请裂变（部分字段与users表的Aff*重复，这里保留额外的）
	InviteRewardTotal decimal.Decimal `json:"invite_reward_total" gorm:"type:decimal(10,6);default:0"`

	// 订阅
	CurrentPlanId  *uint `json:"current_plan_id"`
	SubscriptionId *uint `json:"subscription_id"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (UserExtension) TableName() string {
	return "user_extensions"
}

// ========== 邀请裂变 ==========

// Referral 邀请裂变记录
type Referral struct {
	Id          uint            `json:"id" gorm:"primaryKey"`
	InviterId   uint            `json:"inviter_id" gorm:"index;not null"`
	InviteeId   uint            `json:"invitee_id" gorm:"index;not null"`
	InviteCode  string          `json:"invite_code" gorm:"size:16"`

	// 奖励
	BonusUSD      decimal.Decimal `json:"bonus_usd" gorm:"type:decimal(10,6);default:3.00"`
	InviterBonus  decimal.Decimal `json:"inviter_bonus" gorm:"type:decimal(10,6);default:3.00"`
	InviteeBonus  decimal.Decimal `json:"invitee_bonus" gorm:"type:decimal(10,6);default:3.00"`

	// 状态
	Status      string    `json:"status" gorm:"size:20;default:'pending'"` // pending/granted/rejected/expired
	GrantReason string    `json:"grant_reason" gorm:"size:64"` // first_charge/manual

	// 防刷
	IPAddress         string    `json:"ip_address" gorm:"size:45"`
	DeviceFingerprint string    `json:"device_fingerprint" gorm:"size:64"`

	// 时间戳
	CreatedAt time.Time  `json:"created_at" gorm:"autoCreateTime"`
	GrantedAt *time.Time `json:"granted_at"`
	ExpiredAt *time.Time `json:"expired_at"`
}

// TableName 指定表名
func (Referral) TableName() string {
	return "referrals"
}

// ========== 月度套餐 ==========

// Plan 月度套餐定义
type Plan struct {
	Id          uint   `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"size:50;uniqueIndex;not null"`
	DisplayName string `json:"display_name" gorm:"size:100"`
	Description string `json:"description" gorm:"size:500"`

	// 定价
	MonthlyPrice decimal.Decimal `json:"monthly_price" gorm:"type:decimal(10,2);not null"`
	YearlyPrice  decimal.Decimal `json:"yearly_price" gorm:"type:decimal(10,2)"`

	// 额度
	IncludedUSD  decimal.Decimal `json:"included_usd" gorm:"type:decimal(10,2);not null"`
	DiscountRate decimal.Decimal `json:"discount_rate" gorm:"type:decimal(3,2);default:1.00"` // 0.8 = 8折

	// 权限
	MaxSubAccounts int `json:"max_sub_accounts" gorm:"default:0"`
	MaxAPIKeys     int `json:"max_api_keys" gorm:"default:5"`
	MaxTeamMembers int `json:"max_team_members" gorm:"default:0"`

	// 功能开关（TEXT兼容所有数据库）
	Features string `json:"features" gorm:"type:text"` // JSON格式存储

	// 排序和状态
	SortOrder int  `json:"sort_order" gorm:"default:0"`
	IsActive  bool `json:"is_active" gorm:"default:true"`
	IsDefault bool `json:"is_default" gorm:"default:false"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Plan) TableName() string {
	return "plans"
}

// ========== 订阅 ==========

// Subscription 用户订阅
type Subscription struct {
	Id       uint   `json:"id" gorm:"primaryKey"`
	UserId   uint   `json:"user_id" gorm:"index;not null"`
	PlanId   uint   `json:"plan_id" gorm:"index;not null"`

	// 状态
	Status       string `json:"status" gorm:"size:20;default:'active'"` // active/cancelled/expired/suspended
	BillingCycle string `json:"billing_cycle" gorm:"size:20;default:'monthly'"` // monthly/yearly

	// 额度
	UsedUSD      decimal.Decimal `json:"used_usd" gorm:"type:decimal(12,6);default:0"`
	RemainingUSD decimal.Decimal `json:"remaining_usd" gorm:"type:decimal(12,6)"`

	// 时间
	StartedAt         time.Time  `json:"started_at" gorm:"autoCreateTime"`
	ExpiresAt         time.Time  `json:"expires_at;not null"`
	CancelledAt       *time.Time `json:"cancelled_at"`
	RenewalReminderAt *time.Time `json:"renewal_reminder_at"`

	CancelReason string `json:"cancel_reason" gorm:"size:200"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Subscription) TableName() string {
	return "subscriptions"
}

// ========== 消费记录 ==========

// ConsumptionRecord 消费记录
type ConsumptionRecord struct {
	Id        uint   `json:"id" gorm:"primaryKey"`
	UserId    uint   `json:"user_id" gorm:"index;not null"`
	RequestId string `json:"request_id" gorm:"index;size:64"`

	// 模型信息
	Model     string `json:"model" gorm:"size:64"`
	ChannelId uint   `json:"channel_id"`

	// Token 消耗
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`

	// 计费
	InputPrice    decimal.Decimal `json:"input_price" gorm:"type:decimal(10,6)"` // $/1M tokens
	OutputPrice   decimal.Decimal `json:"output_price" gorm:"type:decimal(10,6)"`
	PreDeductUSD  decimal.Decimal `json:"pre_deduct_usd" gorm:"type:decimal(10,6)"`
	ActualCostUSD decimal.Decimal `json:"actual_cost_usd" gorm:"type:decimal(10,6)"`
	RefundUSD     decimal.Decimal `json:"refund_usd" gorm:"type:decimal(10,6);default:0"`

	// 余额来源
	BalanceSource string `json:"balance_source" gorm:"size:20"` // trial/paid/subscription

	// 状态
	Status string `json:"status" gorm:"size:20;default:'completed'"` // pending/completed/refunded/failed

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定表名
func (ConsumptionRecord) TableName() string {
	return "consumption_records"
}

// ========== 支付记录 ==========

// Payment 支付记录
type Payment struct {
	Id     uint   `json:"id" gorm:"primaryKey"`
	UserId uint   `json:"user_id" gorm:"index;not null"`

	// 支付信息
	Amount        decimal.Decimal `json:"amount" gorm:"type:decimal(10,2);not null"`
	Currency      string          `json:"currency" gorm:"size:3;default:'USD'"`
	PaymentMethod string          `json:"payment_method" gorm:"size:20"` // stripe/alipay/wechat

	// 外部支付ID
	ExternalId string `json:"external_id" gorm:"size:128"` // Stripe PaymentIntent ID

	// 状态
	Status string `json:"status" gorm:"size:20;default:'pending'"` // pending/succeeded/failed/refunded

	// 退款
	RefundAmount decimal.Decimal `json:"refund_amount" gorm:"type:decimal(10,2);default:0"`
	RefundReason string          `json:"refund_reason" gorm:"size:200"`

	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	SucceededAt *time.Time `json:"succeeded_at"`
	RefundedAt  *time.Time `json:"refunded_at"`
}

// TableName 指定表名
func (Payment) TableName() string {
	return "payments"
}

// ========== 定价表 ==========

// ModelPricing 模型定价
type ModelPricing struct {
	Id       uint   `json:"id" gorm:"primaryKey"`
	Model    string `json:"model" gorm:"uniqueIndex;size:64;not null"`
	IsActive bool   `json:"is_active" gorm:"default:true"`

	// Token 定价
	InputPrice  decimal.Decimal `json:"input_price" gorm:"type:decimal(10,6)"`  // $/1M tokens
	OutputPrice decimal.Decimal `json:"output_price" gorm:"type:decimal(10,6)"` // $/1M tokens

	// 图片/视频/语音定价
	PerImagePrice  decimal.Decimal `json:"per_image_price" gorm:"type:decimal(10,6)"`
	PerSecondPrice decimal.Decimal `json:"per_second_price" gorm:"type:decimal(10,6)"` // 视频
	PerCharPrice   decimal.Decimal `json:"per_char_price" gorm:"type:decimal(10,6)"`   // TTS

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ModelPricing) TableName() string {
	return "model_pricing"
}

// ========== 管理员 ==========

// AdminRole 管理员角色类型
type AdminRole string

const (
	RoleSuperAdmin AdminRole = "super_admin"
	RoleOperator   AdminRole = "operator"
	RoleViewer     AdminRole = "viewer"
)

// Admin 管理员表
type Admin struct {
	Id           uint      `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Email        string    `json:"email" gorm:"uniqueIndex;size:128;not null"`
	PasswordHash string    `json:"-" gorm:"size:128;not null"`
	Role         AdminRole `json:"role" gorm:"size:20;default:'operator'"`
	TOTPSecret   string    `json:"-" gorm:"size:64"`
	IsActive     bool      `json:"is_active" gorm:"default:true"`

	CreatedAt   time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

// TableName 指定表名
func (Admin) TableName() string {
	return "admins"
}

// AdminAuditLog 管理员审计日志
type AdminAuditLog struct {
	Id          uint      `json:"id" gorm:"primaryKey"`
	AdminId     uint      `json:"admin_id" gorm:"index;not null"`
	AdminName   string    `json:"admin_name" gorm:"size:64;not null"`
	Operation   string    `json:"operation" gorm:"size:100;not null"`
	TargetId    string    `json:"target_id" gorm:"size:64"`
	TargetType  string    `json:"target_type" gorm:"size:32"`
	OldValue    string    `json:"old_value" gorm:"type:text"`
	NewValue    string    `json:"new_value" gorm:"type:text"`
	IP          string    `json:"ip" gorm:"size:45"`
	UserAgent   string    `json:"user_agent" gorm:"type:text"`
	TOTPVerified bool     `json:"totp_verified" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定表名
func (AdminAuditLog) TableName() string {
	return "admin_audit_logs"
}

// ========== Webhook ==========

// Webhook Webhook 配置
type Webhook struct {
	Id        uint      `json:"id" gorm:"primaryKey"`
	UserId    uint      `json:"user_id" gorm:"index"` // 0 表示系统级 webhook
	Name      string    `json:"name" gorm:"size:64;not null"`
	URL       string    `json:"url" gorm:"size:256;not null"`
	Events    string    `json:"events" gorm:"type:text"`       // 逗号分隔: balance_low,task_complete,error
	Secret    string    `json:"-" gorm:"size:64"`              // 签名密钥
	IsActive  bool      `json:"is_active" gorm:"default:true"`
	LastSent  *time.Time `json:"last_sent"`
	SendCount int64     `json:"send_count" gorm:"default:0"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Webhook) TableName() string {
	return "webhooks"
}

// ========== 预算配置 ==========

// UserBudgetConfig 用户预算配置
type UserBudgetConfig struct {
	Id             uint      `json:"id" gorm:"primaryKey"`
	UserId         uint      `json:"user_id" gorm:"uniqueIndex;not null"`
	DailyLimit     float64   `json:"daily_limit" gorm:"default:0"`      // 日限额 USD, 0 表示无限制
	MonthlyLimit   float64   `json:"monthly_limit" gorm:"default:0"`    // 月限额 USD, 0 表示无限制
	AlertThreshold float64   `json:"alert_threshold" gorm:"default:0.8"` // 告警阈值，默认 80%
	AlertSent      bool      `json:"alert_sent" gorm:"default:false"`   // 本周期是否已发送告警
	NotifyEmail    bool      `json:"notify_email" gorm:"default:true"`  // 邮件通知
	NotifyWebhook  bool      `json:"notify_webhook" gorm:"default:true"` // Webhook 通知
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (UserBudgetConfig) TableName() string {
	return "user_budget_configs"
}

// ========== 默认数据 ==========

// DefaultPlans 默认套餐定义
var DefaultPlans = []Plan{
	{
		Name:         "Starter",
		DisplayName:  "Starter",
		Description:  "适合轻度使用者",
		MonthlyPrice: decimal.NewFromFloat(15.00),
		IncludedUSD:  decimal.NewFromFloat(18.00),
		DiscountRate: decimal.NewFromFloat(1.00),
		MaxAPIKeys:   3,
		SortOrder:    1,
	},
	{
		Name:          "Developer",
		DisplayName:   "Developer",
		Description:   "适合日常开发者",
		MonthlyPrice:  decimal.NewFromFloat(59.00),
		IncludedUSD:   decimal.NewFromFloat(80.00),
		DiscountRate:  decimal.NewFromFloat(0.90),
		MaxAPIKeys:    10,
		SortOrder:     2,
	},
	{
		Name:           "Team",
		DisplayName:    "Team",
		Description:    "适合小团队",
		MonthlyPrice:   decimal.NewFromFloat(199.00),
		IncludedUSD:    decimal.NewFromFloat(300.00),
		DiscountRate:   decimal.NewFromFloat(0.80),
		MaxSubAccounts: 5,
		MaxAPIKeys:     50,
		MaxTeamMembers: 5,
		SortOrder:      3,
	},
	{
		Name:           "Enterprise",
		DisplayName:    "Enterprise",
		Description:    "适合企业客户",
		MonthlyPrice:   decimal.NewFromFloat(0),
		IncludedUSD:    decimal.NewFromFloat(0),
		DiscountRate:   decimal.NewFromFloat(0.75),
		MaxSubAccounts: 100,
		MaxAPIKeys:     500,
		MaxTeamMembers: 100,
		SortOrder:      4,
	},
}

// DefaultModelPricing 默认模型定价
var DefaultModelPricing = []ModelPricing{
	{Model: "claude-opus-4-6", InputPrice: decimal.NewFromFloat(10.00), OutputPrice: decimal.NewFromFloat(50.00)},
	{Model: "claude-sonnet-4-6", InputPrice: decimal.NewFromFloat(2.00), OutputPrice: decimal.NewFromFloat(10.00)},
	{Model: "claude-haiku", InputPrice: decimal.NewFromFloat(0.40), OutputPrice: decimal.NewFromFloat(2.00)},
	{Model: "doubao-seed-1-6", InputPrice: decimal.NewFromFloat(0.10), OutputPrice: decimal.NewFromFloat(0.30)},
	{Model: "doubao-seedream-3-0", PerImagePrice: decimal.NewFromFloat(0.008)},
	{Model: "doubao-video", PerSecondPrice: decimal.NewFromFloat(0.035)},
	{Model: "doubao-tts", PerCharPrice: decimal.NewFromFloat(0.000002)},
}

// ========== 配置 ==========

// TrialConfig 注册赠送配置
type TrialConfig struct {
	TrialAmount decimal.Decimal
	ValidDays   int  // 有效天数（0表示永久）
	AutoGrant   bool // 注册自动发放
}

// DefaultTrialConfig 默认注册赠送配置
var DefaultTrialConfig = TrialConfig{
	TrialAmount: decimal.NewFromFloat(1.00),
	ValidDays:   0,
	AutoGrant:   true,
}

// ReferralConfig 邀请裂变配置
type ReferralConfig struct {
	BonusUSD            decimal.Decimal // 单次奖励金额
	MaxBonusPerUser     decimal.Decimal // 单用户月上限
	IPCooldownHours     int             // 同IP冷却时间（小时）
	DeviceCooldownHours int             // 同设备冷却时间（小时）
	RequireFirstCharge  bool            // 是否需要首次充值才发放
}

// DefaultReferralConfig 默认邀请裂变配置
var DefaultReferralConfig = ReferralConfig{
	BonusUSD:            decimal.NewFromFloat(3.00),
	MaxBonusPerUser:     decimal.NewFromFloat(30.00),
	IPCooldownHours:     24,
	DeviceCooldownHours: 24,
	RequireFirstCharge:  true,
}

// PlaygroundHistory Playground历史记录
type PlaygroundHistory struct {
	Id        uint      `json:"id" gorm:"primaryKey"`
	UserId    uint      `json:"user_id" gorm:"index;not null"`
	Models    string    `json:"models" gorm:"type:text"`      // JSON数组: ["model1", "model2"]
	Prompt    string    `json:"prompt" gorm:"type:text"`      // 用户输入的提示词
	Response  string    `json:"response" gorm:"type:text"`    // JSON数组: 各模型的响应
	CostUSD   string    `json:"cost_usd" gorm:"size:32"`      // 总成本
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定表名
func (PlaygroundHistory) TableName() string {
	return "playground_histories"
}

// GetPlaygroundHistory 获取用户Playground历史记录
func GetPlaygroundHistory(userId uint, limit int) ([]PlaygroundHistory, error) {
	var histories []PlaygroundHistory
	query := DB.Where("user_id = ?", userId).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&histories).Error
	return histories, err
}

// CreatePlaygroundHistory 创建Playground历史记录
func CreatePlaygroundHistory(history *PlaygroundHistory) error {
	return DB.Create(history).Error
}

// DeletePlaygroundHistory 删除Playground历史记录
func DeletePlaygroundHistory(id uint, userId uint) error {
	return DB.Where("id = ? AND user_id = ?", id, userId).Delete(&PlaygroundHistory{}).Error
}
