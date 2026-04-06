// Package controller 提供TT管理后台控制器
package controller

import (
	"net/http"
	"strconv"

	ttmodel "github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ========== 运营看板 ==========

// DashboardData 看板数据
type DashboardData struct {
	TodayRequests    int64   `json:"today_requests"`
	TodayRevenue     string  `json:"today_revenue"`
	TodayCost        string  `json:"today_cost"`
	TodayGrossMargin string  `json:"today_gross_margin"`
	ActiveUsers      int64   `json:"active_users"`
	TotalUsers       int64   `json:"total_users"`
	APIAvailability  string  `json:"api_availability"`
	PoolAvailability string  `json:"pool_availability"`
	RecentErrors     []Error `json:"recent_errors"`
	TrendData        Trend   `json:"trend_data"`
}

// Error 错误信息
type Error struct {
	Time    string `json:"time"`
	Model   string `json:"model"`
	Message string `json:"message"`
}

// Trend 趋势数据
type Trend struct {
	Dates    []string `json:"dates"`
	Requests []int64  `json:"requests"`
	Revenue  []string `json:"revenue"`
}

// GetAdminDashboard 获取运营看板数据
func GetAdminDashboard(c *gin.Context) {
	data, err := ttmodel.GetDashboardData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get dashboard data"})
		return
	}
	c.JSON(http.StatusOK, data)
}

// ========== 用户管理 ==========

// UserListResponse 用户列表响应
type UserListResponse struct {
	Id           uint   `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Balance      string `json:"balance"`
	TotalUsed    string `json:"total_used"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	LastActiveAt string `json:"last_active_at"`
}

// ListUsers 列出用户
func ListUsers(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "20")
	search := c.Query("search")
	status := c.Query("status")

	users, total, err := ttmodel.ListUsersForAdmin(page, pageSize, search, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      users,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetUser 获取用户详情
func GetUser(c *gin.Context) {
	userId := c.Param("id")
	id, err := strconv.Atoi(userId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := ttmodel.GetUserDetailForAdmin(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	Group       string `json:"group"`
}

// UpdateUser 更新用户
func UpdateUser(c *gin.Context) {
	userId := c.Param("id")
	id, err := strconv.Atoi(userId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 记录审计日志
	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "UPDATE_USER", userId, "user", c)

	err = ttmodel.UpdateUserByAdmin(uint(id), req.DisplayName, req.Status, req.Group)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AdjustBalanceRequest 调整余额请求
type AdjustBalanceRequest struct {
	Amount  string `json:"amount" binding:"required"`  // USD金额，正数增加，负数减少
	Reason  string `json:"reason" binding:"required"`
}

// AdjustUserBalance 调整用户余额
func AdjustUserBalance(c *gin.Context) {
	userId := c.Param("id")
	id, err := strconv.Atoi(userId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req AdjustBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount"})
		return
	}

	// 记录审计日志（需要TOTP验证）
	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "ADJUST_BALANCE", userId, "user", c)

	err = ttmodel.AdjustUserBalance(uint(id), amount, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "余额调整成功",
	})
}

// SetUserStatusRequest 设置用户状态请求
type SetUserStatusRequest struct {
	Status string `json:"status" binding:"required"` // active/suspended/banned
	Reason string `json:"reason"`
}

// SetUserStatus 设置用户状态
func SetUserStatus(c *gin.Context) {
	userId := c.Param("id")
	id, err := strconv.Atoi(userId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req SetUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "SET_STATUS", userId, "user", c)

	err = ttmodel.SetUserStatusByAdmin(uint(id), req.Status, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== 渠道管理 ==========

// ChannelListResponse 渠道列表响应
type ChannelListResponse struct {
	Id          uint   `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Priority    int    `json:"priority"`
	SuccessRate string `json:"success_rate"`
	Latency     int64  `json:"latency_ms"`
}

// ListChannels 列出渠道
func ListChannels(c *gin.Context) {
	channels, err := ttmodel.ListChannelsForAdmin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list channels"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": channels})
}

// CreateChannelRequest 创建渠道请求
type CreateChannelRequest struct {
	Name     string `json:"name" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Key      string `json:"key" binding:"required"`
	BaseURL  string `json:"base_url"`
	Models   string `json:"models"`
	Priority int    `json:"priority"`
}

// CreateChannel 创建渠道
func CreateChannel(c *gin.Context) {
	var req CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "CREATE_CHANNEL", "", "channel", c)

	channel, err := ttmodel.CreateChannelByAdmin(req.Name, req.Type, req.Key, req.BaseURL, req.Models, req.Priority)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "channel": channel})
}

// UpdateChannelRequest 更新渠道请求
type UpdateChannelRequest struct {
	Name     string `json:"name"`
	Key      string `json:"key"`
	BaseURL  string `json:"base_url"`
	Models   string `json:"models"`
	Priority int    `json:"priority"`
	Status   string `json:"status"`
}

// UpdateChannel 更新渠道
func UpdateChannel(c *gin.Context) {
	channelId := c.Param("id")
	id, err := strconv.Atoi(channelId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel id"})
		return
	}

	var req UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "UPDATE_CHANNEL", channelId, "channel", c)

	err = ttmodel.UpdateChannelByAdmin(uint(id), req.Name, req.Key, req.BaseURL, req.Models, req.Priority, req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteChannel 删除渠道
func DeleteChannel(c *gin.Context) {
	channelId := c.Param("id")
	id, err := strconv.Atoi(channelId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel id"})
		return
	}

	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "DELETE_CHANNEL", channelId, "channel", c)

	err = ttmodel.DeleteChannelByAdmin(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// TestChannel 测试渠道
func TestChannel(c *gin.Context) {
	channelId := c.Param("id")
	id, err := strconv.Atoi(channelId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel id"})
		return
	}

	result, err := ttmodel.TestChannelByAdmin(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  result,
	})
}

// ========== 号池管理 ==========

// PoolStatus 号池状态
type PoolStatus struct {
	TotalAccounts   int     `json:"total_accounts"`
	Available       int     `json:"available"`
	Cooldown        int     `json:"cooldown"`
	Banned          int     `json:"banned"`
	UtilizationRate string  `json:"utilization_rate"`
}

// GetPoolStatus 获取号池状态
func GetPoolStatus(c *gin.Context) {
	status, err := ttmodel.GetPoolStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get pool status"})
		return
	}
	c.JSON(http.StatusOK, status)
}

// PoolAccount 账号信息
type PoolAccount struct {
	Id           uint   `json:"id"`
	Email        string `json:"email"`
	Status       string `json:"status"` // available/cooldown/banned
	QuotaUsed    string `json:"quota_used"`
	QuotaTotal   string `json:"quota_total"`
	CooldownEnd  string `json:"cooldown_end,omitempty"`
	LastUsed     string `json:"last_used"`
	ProxyIP      string `json:"proxy_ip"`
	RequestCount int64  `json:"request_count"`
}

// ListPoolAccounts 列出号池账号
func ListPoolAccounts(c *gin.Context) {
	status := c.Query("status")
	accounts, err := ttmodel.ListPoolAccounts(status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list accounts"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": accounts})
}

// AddPoolAccountRequest 添加账号请求
type AddPoolAccountRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	ProxyIP  string `json:"proxy_ip"`
}

// AddPoolAccount 添加账号
func AddPoolAccount(c *gin.Context) {
	var req AddPoolAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "ADD_POOL_ACCOUNT", req.Email, "pool", c)

	account, err := ttmodel.AddPoolAccount(req.Email, req.Password, req.ProxyIP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "account": account})
}

// RemovePoolAccount 移除账号
func RemovePoolAccount(c *gin.Context) {
	accountId := c.Param("id")
	id, err := strconv.Atoi(accountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "REMOVE_POOL_ACCOUNT", accountId, "pool", c)

	err = ttmodel.RemovePoolAccount(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RefreshPoolAccount 刷新账号Token
func RefreshPoolAccount(c *gin.Context) {
	accountId := c.Param("id")
	id, err := strconv.Atoi(accountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	err = ttmodel.RefreshPoolAccountToken(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Token刷新成功"})
}

// ========== 定价管理 ==========

// PricingList 定价列表
type PricingList struct {
	Id          uint   `json:"id"`
	Model       string `json:"model"`
	InputPrice  string `json:"input_price"`
	OutputPrice string `json:"output_price"`
	IsActive    bool   `json:"is_active"`
}

// ListPricing 列出定价
func ListPricing(c *gin.Context) {
	pricing, err := ttmodel.ListAllPricing()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list pricing"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": pricing})
}

// CreatePricingRequest 创建定价请求
type CreatePricingRequest struct {
	Model          string `json:"model" binding:"required"`
	InputPrice     string `json:"input_price"`
	OutputPrice    string `json:"output_price"`
	PerImagePrice  string `json:"per_image_price"`
	PerSecondPrice string `json:"per_second_price"`
	PerCharPrice   string `json:"per_char_price"`
}

// CreatePricing 创建定价
func CreatePricing(c *gin.Context) {
	var req CreatePricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "CREATE_PRICING", req.Model, "pricing", c)

	pricing, err := ttmodel.CreateModelPricing(req.Model, req.InputPrice, req.OutputPrice, req.PerImagePrice, req.PerSecondPrice, req.PerCharPrice)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "pricing": pricing})
}

// UpdatePricing 更新定价
func UpdatePricing(c *gin.Context) {
	pricingId := c.Param("id")
	id, err := strconv.Atoi(pricingId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pricing id"})
		return
	}

	var req CreatePricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "UPDATE_PRICING", pricingId, "pricing", c)

	err = ttmodel.UpdateModelPricing(uint(id), req.InputPrice, req.OutputPrice, req.PerImagePrice, req.PerSecondPrice, req.PerCharPrice)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== 套餐管理 ==========

// ListAdminPlans 列出套餐（管理端）
func ListAdminPlans(c *gin.Context) {
	plans, err := ttmodel.GetAllPlans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list plans"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plans})
}

// CreatePlanRequest 创建套餐请求
type CreatePlanRequest struct {
	Name          string `json:"name" binding:"required"`
	DisplayName   string `json:"display_name"`
	Description   string `json:"description"`
	MonthlyPrice  string `json:"monthly_price" binding:"required"`
	IncludedUSD   string `json:"included_usd" binding:"required"`
	DiscountRate  string `json:"discount_rate"`
	MaxAPIKeys    int    `json:"max_api_keys"`
	MaxSubAccounts int   `json:"max_sub_accounts"`
	Features      string `json:"features"`
}

// CreatePlan 创建套餐
func CreatePlan(c *gin.Context) {
	var req CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "CREATE_PLAN", req.Name, "plan", c)

	plan, err := ttmodel.CreatePlanByAdmin(req.Name, req.DisplayName, req.Description, req.MonthlyPrice, req.IncludedUSD, req.DiscountRate, req.MaxAPIKeys, req.MaxSubAccounts, req.Features)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "plan": plan})
}

// UpdatePlan 更新套餐
func UpdatePlan(c *gin.Context) {
	planId := c.Param("id")
	id, err := strconv.Atoi(planId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
		return
	}

	var req CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "UPDATE_PLAN", planId, "plan", c)

	err = ttmodel.UpdatePlanByAdmin(uint(id), req.DisplayName, req.Description, req.MonthlyPrice, req.IncludedUSD, req.DiscountRate, req.MaxAPIKeys, req.MaxSubAccounts, req.Features)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== 财务中心 ==========

// FinanceOverview 财务概览
type FinanceOverview struct {
	TotalRevenue      string `json:"total_revenue"`
	TotalCost         string `json:"total_cost"`
	GrossMargin       string `json:"gross_margin"`
	MonthlyRevenue    string `json:"monthly_revenue"`
	MonthlyCost       string `json:"monthly_cost"`
	PendingWithdrawal string `json:"pending_withdrawal"`
}

// GetFinanceOverview 获取财务概览
func GetFinanceOverview(c *gin.Context) {
	overview, err := ttmodel.GetFinanceOverview()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get finance overview"})
		return
	}
	c.JSON(http.StatusOK, overview)
}

// RevenueReport 收入报告
type RevenueReport struct {
	Period   string         `json:"period"`
	Data     []RevenueItem  `json:"data"`
	ByModel  []RevenueItem  `json:"by_model"`
	BySource []RevenueItem  `json:"by_source"`
}

// RevenueItem 收入项
type RevenueItem struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
}

// GetRevenueReport 获取收入报告
func GetRevenueReport(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	report, err := ttmodel.GetRevenueReport(startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get revenue report"})
		return
	}
	c.JSON(http.StatusOK, report)
}

// GetAdminCostReport 获取管理端成本报告
func GetAdminCostReport(c *gin.Context) {
	req := ttmodel.CostReportRequest{
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
	}

	report, err := ttmodel.GetCostReport(0, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get cost report"})
		return
	}
	c.JSON(http.StatusOK, report)
}

// ListPayments 列出支付记录
func ListPayments(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "20")
	status := c.Query("status")

	payments, total, err := ttmodel.ListPaymentsForAdmin(page, pageSize, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list payments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      payments,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ========== 审计日志 ==========

// AuditLog 审计日志
type AuditLog struct {
	Id          uint   `json:"id"`
	AdminName   string `json:"admin_name"`
	Operation   string `json:"operation"`
	TargetType  string `json:"target_type"`
	TargetId    string `json:"target_id"`
	IP          string `json:"ip"`
	TOTPVerified bool  `json:"totp_verified"`
	CreatedAt   string `json:"created_at"`
}

// ListAuditLogs 列出审计日志
func ListAuditLogs(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("page_size", "20")
	adminId := c.Query("admin_id")
	operation := c.Query("operation")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	logs, total, err := ttmodel.ListAuditLogsForAdmin(page, pageSize, adminId, operation, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ========== 系统设置 ==========

// Settings 系统设置
type Settings struct {
	TrialAmount        string `json:"trial_amount"`
	MinRecharge        string `json:"min_recharge"`
	ReferralBonus      string `json:"referral_bonus"`
	RefundPolicy       string `json:"refund_policy"`
	MaintenanceMode    bool   `json:"maintenance_mode"`
	RegistrationOpen   bool   `json:"registration_open"`
}

// GetSettings 获取系统设置
func GetSettings(c *gin.Context) {
	settings, err := ttmodel.GetSystemSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// UpdateSettings 更新系统设置
func UpdateSettings(c *gin.Context) {
	var req Settings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	adminId := c.GetInt("admin_id")
	ttmodel.RecordAdminAudit(adminId, "UPDATE_SETTINGS", "", "settings", c)

	err := ttmodel.UpdateSystemSettings(ttmodel.Settings(req))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== Webhook管理 ==========

// Webhook Webhook信息
type Webhook struct {
	Id        uint   `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Events    string `json:"events"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

// ListWebhooks 列出Webhooks
func ListWebhooks(c *gin.Context) {
	webhooks, err := ttmodel.ListWebhooksForAdmin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list webhooks"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": webhooks})
}

// CreateWebhookRequest 创建Webhook请求
type CreateWebhookRequest struct {
	Name   string `json:"name" binding:"required"`
	URL    string `json:"url" binding:"required"`
	Events string `json:"events" binding:"required"` // comma separated: balance_low,task_complete,error
}

// CreateWebhook 创建Webhook
func CreateWebhook(c *gin.Context) {
	var req CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	webhook, err := ttmodel.CreateWebhookByAdmin(req.Name, req.URL, req.Events)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "webhook": webhook})
}

// UpdateWebhook 更新Webhook
func UpdateWebhook(c *gin.Context) {
	webhookId := c.Param("id")
	id, err := strconv.Atoi(webhookId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook id"})
		return
	}

	var req CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	err = ttmodel.UpdateWebhookByAdmin(uint(id), req.Name, req.URL, req.Events)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteWebhook 删除Webhook
func DeleteWebhook(c *gin.Context) {
	webhookId := c.Param("id")
	id, err := strconv.Atoi(webhookId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook id"})
		return
	}

	err = ttmodel.DeleteWebhookByAdmin(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// TestWebhook 测试Webhook
func TestWebhook(c *gin.Context) {
	webhookId := c.Param("id")
	id, err := strconv.Atoi(webhookId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook id"})
		return
	}

	result, err := ttmodel.TestWebhookByAdmin(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  result,
	})
}
