//go:build tt
// +build tt

package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// SLAConfig SLA 配置
type SLAConfig struct {
	Id        uint      `json:"id" gorm:"primaryKey"`
	UserId    *uint     `json:"user_id" gorm:"index"` // 用户级别 SLA
	TeamId    *uint     `json:"team_id" gorm:"index"` // 团队级别 SLA

	// SLA 级别
	SLATier    string `json:"sla_tier" gorm:"size:20;not null"` // basic/pro/enterprise

	// 可用率承诺
	TargetAvailability float64 `json:"target_availability"` // 目标可用率，如 99.5
	MonthlyUptimeMin   float64 `json:"monthly_uptime_min"`  // 月度最低在线时间（分钟）

	// 响应时间承诺
	MaxLatencyMs       int64   `json:"max_latency_ms"`        // 最大延迟承诺
	AvgLatencyTarget   float64 `json:"avg_latency_target"`     // 平均延迟目标

	// 故障响应时间
	P0ResponseMin      int   `json:"p0_response_min"`       // P0 级故障响应时间
	P1ResponseMin      int   `json:"p1_response_min"`       // P1 级故障响应时间
	P2ResponseMin      int   `json:"p2_response_min"`       // P2 级故障响应时间

	// 赔偿条款
	CreditPercent       float64 `json:"credit_percent"`        // 违约赔偿百分比
	MaxCreditPercent     float64 `json:"max_credit_percent"`    // 最大赔偿百分比
	CreditAutoApply      bool    `json:"credit_auto_apply"`      // 自动应用赔偿

	// 监控配置
	CheckIntervalSec    int    `json:"check_interval_sec"`     // 健康检查间隔
	AlertThreshold      float64 `json:"alert_threshold"`       // 告警阈值（可用率低于此值告警）
	IncludeMaintenance   bool    `json:"include_maintenance"`  // 是否计入维护时间

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (SLAConfig) TableName() string {
	return "sla_configs"
}

// SLAReport SLA 报告
type SLAReport struct {
	Id          uint      `json:"id" gorm:"primaryKey"`
	ConfigId    uint      `json:"config_id" gorm:"index;not null"`
	UserId      uint      `json:"user_id" gorm:"index"`

	// 报告周期
	ReportType  string    `json:"report_type" gorm:"size:20"` // daily/weekly/monthly
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`

	// 可用率统计
	TotalMinutes       float64 `json:"total_minutes"`        // 总分钟数
	UptimeMinutes      float64 `json:"uptime_minutes"`       // 在线分钟数
	DowntimeMinutes    float64 `json:"downtime_minutes"`     // 离线分钟数
	MaintenanceMinutes float64 `json:"maintenance_minutes"`  // 维护分钟数
	MeasuredAvailability float64 `json:"measured_availability"` // 实际可用率
	TargetAvailability  float64 `json:"target_availability"`   // 目标可用率
	AvailabilityMet      bool    `json:"availability_met"`      // 是否达标

	// 延迟统计
	AvgLatencyMs      float64 `json:"avg_latency_ms"`      // 平均延迟
	MaxLatencyMs      int64   `json:"max_latency_ms"`      // 最大延迟
	P95LatencyMs      float64 `json:"p95_latency_ms"`      // P95 延迟
	LatencyMet        bool    `json:"latency_met"`         // 延迟是否达标

	// 故障统计
	TotalIncidents    int `json:"total_incidents"`    // 总故障数
	P0Incidents       int `json:"p0_incidents"`       // P0 级故障数
	P1Incidents       int `json:"p1_incidents"`       // P1 级故障数
	P2Incidents       int `json:"p2_incidents"`       // P2 级故障数
	AvgResolutionMin  float64 `json:"avg_resolution_min"` // 平均恢复时间

	// 赔偿计算
	IsViolation       bool            `json:"is_violation"`        // 是否违约
	ViolationMinutes float64         `json:"violation_minutes"`   // 违约分钟数
	CreditAmount     decimal.Decimal `json:"credit_amount"`       // 赔偿金额
	CreditApplied    bool            `json:"credit_applied"`      // 是否已应用赔偿
	AppliedAt        *time.Time      `json:"applied_at"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定表名
func (SLAReport) TableName() string {
	return "sla_reports"
}

// SLAIncident SLA 故障事件
type SLAIncident struct {
	Id           uint      `json:"id" gorm:"primaryKey"`
	ConfigId     uint      `json:"config_id" gorm:"index"`

	// 事件信息
	Severity     string    `json:"severity" gorm:"size:10"` // P0/P1/P2
	Title        string    `json:"title" gorm:"size:200"`
	Description  string    `json:"description" gorm:"type:text"`

	// 时间记录
	DetectedAt   time.Time  `json:"detected_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	ResolvedAt   *time.Time `json:"resolved_at"`

	// 影响评估
	AffectedServices []string `json:"affected_services" gorm:"type:text"` // JSON 存储
	AffectedUsers     int     `json:"affected_users"`

	// 状态
	Status       string    `json:"status" gorm:"size:20"` // investigating/identified/monitoring/resolved

	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (SLAIncident) TableName() string {
	return "sla_incidents"
}

// SLABreach SLA 违约记录
type SLABreach struct {
	Id          uint            `json:"id" gorm:"primaryKey"`
	ReportId    uint            `json:"report_id" gorm:"index"`
	UserId      uint            `json:"user_id" gorm:"index"`

	BreachType  string          `json:"breach_type" gorm:"size:50"` // availability/latency/response_time
	Threshold  float64         `json:"threshold"`                  // 阈值
	Measured   float64         `json:"measured"`                   // 实际测量值
	Duration   float64         `json:"duration"`                   // 持续时间（分钟）

	CreditPercent float64         `json:"credit_percent"`  // 赔偿百分比
	CreditAmount  decimal.Decimal `json:"credit_amount"`   // 赔偿金额
	Applied       bool            `json:"applied"`         // 是否已应用

	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定表名
func (SLABreach) TableName() string {
	return "sla_breaches"
}

// SLATier SLA 级别定义
type SLATier struct {
	Name                 string  `json:"name"`
	TargetAvailability   float64 `json:"target_availability"`
	MaxLatencyMs         int64   `json:"max_latency_ms"`
	P0ResponseMin        int     `json:"p0_response_min"`
	P1ResponseMin        int     `json:"p1_response_min"`
	P2ResponseMin        int     `json:"p2_response_min"`
	CreditPercent       float64 `json:"credit_percent"`
	MaxCreditPercent    float64 `json:"max_credit_percent"`
}

// 预定义 SLA 级别
var SLATiers = map[string]SLATier{
	"basic": {
		Name:               "Basic",
		TargetAvailability: 99.0,
		MaxLatencyMs:       500,
		P0ResponseMin:      60,
		P1ResponseMin:      240,
		P2ResponseMin:      1440,
		CreditPercent:      5.0,
		MaxCreditPercent:   25.0,
	},
	"pro": {
		Name:               "Pro",
		TargetAvailability: 99.5,
		MaxLatencyMs:       300,
		P0ResponseMin:      30,
		P1ResponseMin:      120,
		P2ResponseMin:      720,
		CreditPercent:      10.0,
		MaxCreditPercent:   50.0,
	},
	"enterprise": {
		Name:               "Enterprise",
		TargetAvailability: 99.9,
		MaxLatencyMs:       200,
		P0ResponseMin:      15,
		P1ResponseMin:      60,
		P2ResponseMin:      360,
		CreditPercent:      15.0,
		MaxCreditPercent:   100.0,
	},
}

// SLAStatusResponse SLA 状态响应
type SLAStatusResponse struct {
	CurrentAvailability  float64   `json:"current_availability"`
	TargetAvailability   float64   `json:"target_availability"`
	Status              string    `json:"status"` // meeting/at_risk/breached
	CurrentLatencyMs    float64   `json:"current_latency_ms"`
	TargetLatencyMs     int64     `json:"target_latency_ms"`
	NextReportDate      string    `json:"next_report_date"`
	RecentBreaches      int       `json:"recent_breaches"`
	YTDCredits          string    `json:"ytd_credits"`
}

// GetSLAConfig 获取 SLA 配置
func GetSLAConfig(userId uint) (*SLAConfig, error) {
	var config SLAConfig
	err := DB.Where("user_id = ? OR user_id IS NULL", userId).
		Order("user_id DESC").
		First(&config).Error
	if err != nil {
		// 返回默认配置
		tier := SLATiers["basic"]
		return &SLAConfig{
			SLATier:            tier.Name,
			TargetAvailability: tier.TargetAvailability,
			MaxLatencyMs:       tier.MaxLatencyMs,
			CreditPercent:      tier.CreditPercent,
		}, nil
	}
	return &config, nil
}

// GetSLAReport 获取 SLA 报告
func GetSLAReport(configId uint, reportType string, periodStart, periodEnd time.Time) (*SLAReport, error) {
	var report SLAReport
	err := DB.Where("config_id = ? AND report_type = ? AND period_start >= ? AND period_end <= ?",
		configId, reportType, periodStart, periodEnd).
		First(&report).Error
	if err != nil {
		return nil, err
	}
	return &report, nil
}

// CreateSLAReport 创建 SLA 报告
func CreateSLAReport(report *SLAReport) error {
	return DB.Create(report).Error
}

// GetSLABreaches 获取违约记录
func GetSLABreaches(userId uint, limit int) ([]SLABreach, error) {
	var breaches []SLABreach
	query := DB.Where("user_id = ?", userId).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&breaches).Error
	return breaches, err
}

// CalculateSLACredit 计算 SLA 赔偿
func CalculateSLACredit(report *SLAReport) decimal.Decimal {
	if !report.IsViolation || report.MeasuredAvailability >= report.TargetAvailability {
		return decimal.Zero
	}

	// 计算违约百分比
	breachPercent := report.TargetAvailability - report.MeasuredAvailability

	// 获取配置
	config, err := GetSLAConfig(report.UserId)
	if err != nil {
		return decimal.Zero
	}

	// 计算赔偿
	creditPercent := config.CreditPercent
	if breachPercent > 1.0 {
		creditPercent = config.MaxCreditPercent
	} else if breachPercent > 0.5 {
		creditPercent = config.CreditPercent * 2
	}

	// 假设月消费金额（需要从实际数据计算）
	monthlySpend := decimal.NewFromFloat(100.0) // TODO: 从数据库获取实际消费

	return monthlySpend.Mul(decimal.NewFromFloat(creditPercent / 100))
}
