// Package controller 提供TT API控制器
// sla.go - SLA 保障控制器
package controller

import (
	"net/http"
	"strconv"
	"time"

	ttmodel "github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// ========== SLA 状态 ==========

// GetSLAStatus 获取当前 SLA 状态
func GetSLAStatus(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// 获取 SLA 配置
	config, err := ttmodel.GetSLAConfig(uint(userId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get SLA config"})
		return
	}

	// 获取当前可用率（从最近报告计算）
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	report, err := ttmodel.GetSLAReport(config.Id, "monthly", monthStart, now)

	var currentAvailability float64 = 99.9 // 默认值
	var currentLatencyMs float64 = 150     // 默认值
	var recentBreaches int = 0
	var ytdCredits string = "0.00"

	if err == nil && report != nil {
		currentAvailability = report.MeasuredAvailability
		currentLatencyMs = report.AvgLatencyMs
	}

	// 获取违约记录
	breaches, err := ttmodel.GetSLABreaches(uint(userId), 30)
	if err == nil {
		recentBreaches = len(breaches)
		for _, b := range breaches {
			if b.Applied {
				ytdCredits = b.CreditAmount.StringFixed(2)
			}
		}
	}

	// 计算状态
	status := "meeting"
	if currentAvailability < config.TargetAvailability {
		if currentAvailability < config.TargetAvailability-0.5 {
			status = "breached"
		} else {
			status = "at_risk"
		}
	}

	// 下次报告日期（下月第一天）
	nextReport := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())

	c.JSON(http.StatusOK, ttmodel.SLAStatusResponse{
		CurrentAvailability: currentAvailability,
		TargetAvailability: config.TargetAvailability,
		Status:            status,
		CurrentLatencyMs:   currentLatencyMs,
		TargetLatencyMs:    config.MaxLatencyMs,
		NextReportDate:     nextReport.Format("2006-01-02"),
		RecentBreaches:     recentBreaches,
		YTDCredits:         ytdCredits,
	})
}

// ========== SLA 报告 ==========

// SLAReportListResponse SLA 报告列表响应
type SLAReportListResponse struct {
	Id          uint      `json:"id"`
	ReportType  string    `json:"report_type"`
	PeriodStart string    `json:"period_start"`
	PeriodEnd   string    `json:"period_end"`
	Availability float64  `json:"availability"`
	IsViolation  bool      `json:"is_violation"`
	CreatedAt   string    `json:"created_at"`
}

// GetSLAReports 获取 SLA 报告列表
func GetSLAReports(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	period := c.DefaultQuery("period", "monthly")
	limit := 12 // 默认返回最近 12 个报告

	config, err := ttmodel.GetSLAConfig(uint(userId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get SLA config"})
		return
	}

	// 查询报告
	var reports []ttmodel.SLAReport
	query := ttmodel.DB.Where("config_id = ?", config.Id)
	if period != "" {
		query = query.Where("report_type = ?", period)
	}
	query = query.Order("period_end DESC").Limit(limit)

	if err := query.Find(&reports).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get reports"})
		return
	}

	result := make([]SLAReportListResponse, len(reports))
	for i, r := range reports {
		result[i] = SLAReportListResponse{
			Id:           r.Id,
			ReportType:   r.ReportType,
			PeriodStart:  r.PeriodStart.Format("2006-01-02"),
			PeriodEnd:    r.PeriodEnd.Format("2006-01-02"),
			Availability: r.MeasuredAvailability,
			IsViolation:  r.IsViolation,
			CreatedAt:    r.CreatedAt.Format("2006-01-02"),
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// SLAReportDetailResponse SLA 报告详情响应
type SLAReportDetailResponse struct {
	Id                   uint      `json:"id"`
	ReportType           string    `json:"report_type"`
	PeriodStart          string    `json:"period_start"`
	PeriodEnd            string    `json:"period_end"`
	TotalMinutes         float64   `json:"total_minutes"`
	UptimeMinutes        float64   `json:"uptime_minutes"`
	DowntimeMinutes      float64   `json:"downtime_minutes"`
	MaintenanceMinutes   float64   `json:"maintenance_minutes"`
	MeasuredAvailability float64   `json:"measured_availability"`
	TargetAvailability   float64   `json:"target_availability"`
	AvailabilityMet       bool      `json:"availability_met"`
	AvgLatencyMs         float64   `json:"avg_latency_ms"`
	MaxLatencyMs         int64     `json:"max_latency_ms"`
	P95LatencyMs         float64   `json:"p95_latency_ms"`
	LatencyMet           bool      `json:"latency_met"`
	TotalIncidents       int       `json:"total_incidents"`
	P0Incidents          int       `json:"p0_incidents"`
	P1Incidents          int       `json:"p1_incidents"`
	P2Incidents          int       `json:"p2_incidents"`
	AvgResolutionMin     float64   `json:"avg_resolution_min"`
	IsViolation          bool      `json:"is_violation"`
	ViolationMinutes     float64   `json:"violation_minutes"`
	CreditAmount         string    `json:"credit_amount"`
	CreditApplied        bool      `json:"credit_applied"`
	CreatedAt            string    `json:"created_at"`
}

// GetSLAReportDetail 获取 SLA 报告详情
func GetSLAReportDetail(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	reportId := c.Param("id")
	id, err := strconv.Atoi(reportId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report id"})
		return
	}

	var report ttmodel.SLAReport
	if err := ttmodel.DB.First(&report, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return
	}

	// 验证用户权限
	config, err := ttmodel.GetSLAConfig(uint(userId))
	if err != nil || config.Id != report.ConfigId {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.JSON(http.StatusOK, SLAReportDetailResponse{
		Id:                   report.Id,
		ReportType:           report.ReportType,
		PeriodStart:          report.PeriodStart.Format("2006-01-02"),
		PeriodEnd:            report.PeriodEnd.Format("2006-01-02"),
		TotalMinutes:         report.TotalMinutes,
		UptimeMinutes:        report.UptimeMinutes,
		DowntimeMinutes:      report.DowntimeMinutes,
		MaintenanceMinutes:   report.MaintenanceMinutes,
		MeasuredAvailability: report.MeasuredAvailability,
		TargetAvailability:   report.TargetAvailability,
		AvailabilityMet:       report.AvailabilityMet,
		AvgLatencyMs:         report.AvgLatencyMs,
		MaxLatencyMs:         report.MaxLatencyMs,
		P95LatencyMs:         report.P95LatencyMs,
		LatencyMet:           report.LatencyMet,
		TotalIncidents:       report.TotalIncidents,
		P0Incidents:          report.P0Incidents,
		P1Incidents:          report.P1Incidents,
		P2Incidents:          report.P2Incidents,
		AvgResolutionMin:     report.AvgResolutionMin,
		IsViolation:          report.IsViolation,
		ViolationMinutes:     report.ViolationMinutes,
		CreditAmount:         report.CreditAmount.StringFixed(2),
		CreditApplied:        report.CreditApplied,
		CreatedAt:            report.CreatedAt.Format("2006-01-02"),
	})
}

// ========== SLA 违约记录 ==========

// SLABreachResponse SLA 违约响应
type SLABreachResponse struct {
	Id            uint    `json:"id"`
	BreachType    string  `json:"breach_type"`
	Threshold     float64 `json:"threshold"`
	Measured      float64 `json:"measured"`
	Duration      float64 `json:"duration_minutes"`
	CreditPercent float64 `json:"credit_percent"`
	CreditAmount  string  `json:"credit_amount"`
	Applied       bool    `json:"applied"`
	CreatedAt     string  `json:"created_at"`
}

// GetSLABreaches 获取违约记录
func GetSLABreaches(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit := 50 // 默认返回最近 50 条

	breaches, err := ttmodel.GetSLABreaches(uint(userId), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get breaches"})
		return
	}

	result := make([]SLABreachResponse, len(breaches))
	for i, b := range breaches {
		result[i] = SLABreachResponse{
			Id:            b.Id,
			BreachType:    b.BreachType,
			Threshold:     b.Threshold,
			Measured:      b.Measured,
			Duration:      b.Duration,
			CreditPercent: b.CreditPercent,
			CreditAmount:  b.CreditAmount.StringFixed(2),
			Applied:       b.Applied,
			CreatedAt:     b.CreatedAt.Format("2006-01-02"),
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ========== SLA 故障事件 ==========

// SLAIncidentResponse SLA 故障事件响应
type SLAIncidentResponse struct {
	Id               uint      `json:"id"`
	Severity         string    `json:"severity"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	DetectedAt       string    `json:"detected_at"`
	AcknowledgedAt   *string   `json:"acknowledged_at,omitempty"`
	ResolvedAt       *string   `json:"resolved_at,omitempty"`
	AffectedServices []string  `json:"affected_services"`
	AffectedUsers    int       `json:"affected_users"`
	Status           string    `json:"status"`
	CreatedAt        string    `json:"created_at"`
	UpdatedAt        string    `json:"updated_at"`
}

// GetSLAIncidents 获取故障事件列表
func GetSLAIncidents(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	severity := c.Query("severity")
	status := c.Query("status")
	limit := 30 // 默认返回最近 30 条

	config, err := ttmodel.GetSLAConfig(uint(userId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get SLA config"})
		return
	}

	var incidents []ttmodel.SLAIncident
	query := ttmodel.DB.Where("config_id = ?", config.Id)
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("detected_at DESC").Limit(limit)

	if err := query.Find(&incidents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get incidents"})
		return
	}

	result := make([]SLAIncidentResponse, len(incidents))
	for i, inc := range incidents {
		var acknowledgedAt, resolvedAt *string
		if inc.AcknowledgedAt != nil {
			t := inc.AcknowledgedAt.Format(time.RFC3339)
			acknowledgedAt = &t
		}
		if inc.ResolvedAt != nil {
			t := inc.ResolvedAt.Format(time.RFC3339)
			resolvedAt = &t
		}

		result[i] = SLAIncidentResponse{
			Id:               inc.Id,
			Severity:         inc.Severity,
			Title:            inc.Title,
			Description:      inc.Description,
			DetectedAt:       inc.DetectedAt.Format(time.RFC3339),
			AcknowledgedAt:   acknowledgedAt,
			ResolvedAt:       resolvedAt,
			AffectedServices: inc.AffectedServices,
			AffectedUsers:    inc.AffectedUsers,
			Status:           inc.Status,
			CreatedAt:        inc.CreatedAt.Format(time.RFC3339),
			UpdatedAt:        inc.UpdatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// ========== SLA 配置管理 ==========

// SLAConfigResponse SLA 配置响应
type SLAConfigResponse struct {
	Id                 uint    `json:"id"`
	SLATier            string  `json:"sla_tier"`
	TargetAvailability float64 `json:"target_availability"`
	MaxLatencyMs       int64   `json:"max_latency_ms"`
	P0ResponseMin      int     `json:"p0_response_min"`
	P1ResponseMin      int     `json:"p1_response_min"`
	P2ResponseMin      int     `json:"p2_response_min"`
	CreditPercent      float64 `json:"credit_percent"`
	MaxCreditPercent   float64 `json:"max_credit_percent"`
	CreditAutoApply    bool    `json:"credit_auto_apply"`
	CheckIntervalSec   int     `json:"check_interval_sec"`
	AlertThreshold     float64 `json:"alert_threshold"`
	IncludeMaintenance bool    `json:"include_maintenance"`
}

// GetSLAConfigAPI 获取 SLA 配置
func GetSLAConfigAPI(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	config, err := ttmodel.GetSLAConfig(uint(userId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get SLA config"})
		return
	}

	c.JSON(http.StatusOK, SLAConfigResponse{
		Id:                 config.Id,
		SLATier:            config.SLATier,
		TargetAvailability: config.TargetAvailability,
		MaxLatencyMs:       config.MaxLatencyMs,
		P0ResponseMin:      config.P0ResponseMin,
		P1ResponseMin:      config.P1ResponseMin,
		P2ResponseMin:      config.P2ResponseMin,
		CreditPercent:      config.CreditPercent,
		MaxCreditPercent:   config.MaxCreditPercent,
		CreditAutoApply:    config.CreditAutoApply,
		CheckIntervalSec:   config.CheckIntervalSec,
		AlertThreshold:     config.AlertThreshold,
		IncludeMaintenance: config.IncludeMaintenance,
	})
}

// UpdateSLAConfigRequest 更新 SLA 配置请求
type UpdateSLAConfigRequest struct {
	SLATier            *string  `json:"sla_tier"`
	TargetAvailability *float64 `json:"target_availability"`
	MaxLatencyMs       *int64   `json:"max_latency_ms"`
	P0ResponseMin      *int     `json:"p0_response_min"`
	P1ResponseMin      *int     `json:"p1_response_min"`
	P2ResponseMin      *int     `json:"p2_response_min"`
	CreditPercent      *float64 `json:"credit_percent"`
	MaxCreditPercent   *float64 `json:"max_credit_percent"`
	CreditAutoApply    *bool    `json:"credit_auto_apply"`
	CheckIntervalSec   *int     `json:"check_interval_sec"`
	AlertThreshold     *float64 `json:"alert_threshold"`
	IncludeMaintenance *bool    `json:"include_maintenance"`
}

// UpdateSLAConfigAPI 更新 SLA 配置
func UpdateSLAConfigAPI(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req UpdateSLAConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	config, err := ttmodel.GetSLAConfig(uint(userId))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get SLA config"})
		return
	}

	// 如果更新 SLA 级别，使用预定义配置
	if req.SLATier != nil {
		tier, ok := ttmodel.SLATiers[*req.SLATier]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"type":    "invalid_tier",
					"message": "invalid SLA tier, must be one of: basic, pro, enterprise",
				},
			})
			return
		}

		config.SLATier = tier.Name
		config.TargetAvailability = tier.TargetAvailability
		config.MaxLatencyMs = tier.MaxLatencyMs
		config.P0ResponseMin = tier.P0ResponseMin
		config.P1ResponseMin = tier.P1ResponseMin
		config.P2ResponseMin = tier.P2ResponseMin
		config.CreditPercent = tier.CreditPercent
		config.MaxCreditPercent = tier.MaxCreditPercent
	} else {
		// 只更新指定字段
		if req.TargetAvailability != nil {
			config.TargetAvailability = *req.TargetAvailability
		}
		if req.MaxLatencyMs != nil {
			config.MaxLatencyMs = *req.MaxLatencyMs
		}
		if req.P0ResponseMin != nil {
			config.P0ResponseMin = *req.P0ResponseMin
		}
		if req.P1ResponseMin != nil {
			config.P1ResponseMin = *req.P1ResponseMin
		}
		if req.P2ResponseMin != nil {
			config.P2ResponseMin = *req.P2ResponseMin
		}
		if req.CreditPercent != nil {
			config.CreditPercent = *req.CreditPercent
		}
		if req.MaxCreditPercent != nil {
			config.MaxCreditPercent = *req.MaxCreditPercent
		}
	}

	// 通用字段
	if req.CreditAutoApply != nil {
		config.CreditAutoApply = *req.CreditAutoApply
	}
	if req.CheckIntervalSec != nil {
		config.CheckIntervalSec = *req.CheckIntervalSec
	}
	if req.AlertThreshold != nil {
		config.AlertThreshold = *req.AlertThreshold
	}
	if req.IncludeMaintenance != nil {
		config.IncludeMaintenance = *req.IncludeMaintenance
	}

	// 保存配置
	if err := ttmodel.DB.Save(config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update SLA config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"config": SLAConfigResponse{
			Id:                 config.Id,
			SLATier:            config.SLATier,
			TargetAvailability: config.TargetAvailability,
			MaxLatencyMs:       config.MaxLatencyMs,
			P0ResponseMin:      config.P0ResponseMin,
			P1ResponseMin:      config.P1ResponseMin,
			P2ResponseMin:      config.P2ResponseMin,
			CreditPercent:      config.CreditPercent,
			MaxCreditPercent:   config.MaxCreditPercent,
			CreditAutoApply:    config.CreditAutoApply,
			CheckIntervalSec:   config.CheckIntervalSec,
			AlertThreshold:     config.AlertThreshold,
			IncludeMaintenance: config.IncludeMaintenance,
		},
	})
}

// ========== SLA 级别列表 ==========

// SLATierInfo SLA 级别信息
type SLATierInfo struct {
	Name               string  `json:"name"`
	TargetAvailability float64 `json:"target_availability"`
	MaxLatencyMs       int64   `json:"max_latency_ms"`
	P0ResponseMin      int     `json:"p0_response_min"`
	CreditPercent      float64 `json:"credit_percent"`
	MaxCreditPercent   float64 `json:"max_credit_percent"`
}

// GetSLATiers 获取可用的 SLA 级别
func GetSLATiers(c *gin.Context) {
	tiers := make([]SLATierInfo, 0, len(ttmodel.SLATiers))
	for _, tier := range []string{"basic", "pro", "enterprise"} {
		t := ttmodel.SLATiers[tier]
		tiers = append(tiers, SLATierInfo{
			Name:               t.Name,
			TargetAvailability: t.TargetAvailability,
			MaxLatencyMs:       t.MaxLatencyMs,
			P0ResponseMin:      t.P0ResponseMin,
			CreditPercent:      t.CreditPercent,
			MaxCreditPercent:   t.MaxCreditPercent,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": tiers})
}
