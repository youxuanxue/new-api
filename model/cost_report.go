// Package model 提供TT数据模型
// cost_report.go - 成本分析报告模型
package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// CostReport 成本报告
type CostReport struct {
	Id        uint      `json:"id" gorm:"primaryKey"`
	UserId    uint      `json:"user_id" gorm:"index;not null"`
	TeamId    *uint     `json:"team_id" gorm:"index"` // 团队报告时使用

	// 时间维度
	ReportType  string    `json:"report_type" gorm:"size:20;not null"` // daily/weekly/monthly/custom
	StartDate   time.Time `json:"start_date" gorm:"not null"`
	EndDate     time.Time `json:"end_date" gorm:"not null"`

	// 总消费
	TotalCost     decimal.Decimal `json:"total_cost" gorm:"type:decimal(12,6)"`
	TotalRequests int64           `json:"total_requests"`
	TotalTokens   int64           `json:"total_tokens"`
	InputTokens   int64           `json:"input_tokens"`
	OutputTokens  int64           `json:"output_tokens"`

	// 按模型拆解（JSON 存储）
	ModelBreakdown string `json:"model_breakdown" gorm:"type:text"`

	// 按项目拆解（JSON 存储）
	ProjectBreakdown string `json:"project_breakdown" gorm:"type:text"`

	// 按时段拆解（JSON 存储）
	TimeBreakdown string `json:"time_breakdown" gorm:"type:text"`

	// 节省分析
	SavingsVsOfficial decimal.Decimal `json:"savings_vs_official" gorm:"type:decimal(12,6)"` // 相比官方节省
	SavingsPercent    float64         `json:"savings_percent"`                                // 节省百分比

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName 指定表名
func (CostReport) TableName() string {
	return "cost_reports"
}

// ModelCostBreakdown 模型成本拆解
type ModelCostBreakdown struct {
	Model         string          `json:"model"`
	Requests      int64           `json:"requests"`
	InputTokens   int64           `json:"input_tokens"`
	OutputTokens  int64           `json:"output_tokens"`
	Cost          decimal.Decimal `json:"cost"`
	PercentOfTotal float64        `json:"percent_of_total"`
}

// TimeCostBreakdown 时段成本拆解
type TimeCostBreakdown struct {
	Date         string          `json:"date"`
	Requests     int64           `json:"requests"`
	Cost         decimal.Decimal `json:"cost"`
	PercentOfTotal float64       `json:"percent_of_total"`
}

// ProjectCostBreakdown 项目成本拆解
type ProjectCostBreakdown struct {
	ProjectName  string          `json:"project_name"`
	Requests     int64           `json:"requests"`
	Cost         decimal.Decimal `json:"cost"`
	PercentOfTotal float64       `json:"percent_of_total"`
}

// CostReportRequest 成本报告请求
type CostReportRequest struct {
	StartDate   string `json:"start_date" form:"start_date"`     // YYYY-MM-DD
	EndDate     string `json:"end_date" form:"end_date"`         // YYYY-MM-DD
	ReportType  string `json:"report_type" form:"report_type"`   // daily/weekly/monthly/custom
	TeamId      *uint  `json:"team_id" form:"team_id"`
	ProjectName string `json:"project_name" form:"project_name"` // 可选：按项目筛选
	Model       string `json:"model" form:"model"`               // 可选：按模型筛选
}

// CostReportResponse 成本报告响应
type CostReportResponse struct {
	ReportType     string                `json:"report_type"`
	StartDate      string                `json:"start_date"`
	EndDate        string                `json:"end_date"`
	TotalCost      string                `json:"total_cost"`
	TotalRequests  int64                 `json:"total_requests"`
	TotalTokens    int64                 `json:"total_tokens"`
	InputTokens    int64                 `json:"input_tokens"`
	OutputTokens   int64                 `json:"output_tokens"`
	ModelBreakdown []ModelCostBreakdown  `json:"model_breakdown"`
	TimeBreakdown  []TimeCostBreakdown   `json:"time_breakdown"`
	Savings        SavingsInfo           `json:"savings"`
}

// SavingsInfo 节省信息
type SavingsInfo struct {
	VsOfficial   string  `json:"vs_official"`
	Percent      float64 `json:"percent"`
	OfficialCost string  `json:"official_cost"`
}

// GetCostReport 获取成本报告
func GetCostReport(userId uint, req CostReportRequest) (*CostReportResponse, error) {
	// 解析日期
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		startDate = time.Now().AddDate(0, -1, 0)
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		endDate = time.Now()
	}

	// 从消费记录计算报告
	report := &CostReportResponse{
		ReportType:     req.ReportType,
		StartDate:      startDate.Format("2006-01-02"),
		EndDate:        endDate.Format("2006-01-02"),
		TotalCost:      "0.00",
		TotalRequests:  0,
		TotalTokens:    0,
		InputTokens:    0,
		OutputTokens:   0,
		ModelBreakdown: []ModelCostBreakdown{},
		TimeBreakdown:  []TimeCostBreakdown{},
		Savings: SavingsInfo{
			VsOfficial:   "0.00",
			Percent:      0.33, // TT 默认比官方便宜 33%
			OfficialCost: "0.00",
		},
	}

	// TODO: 从数据库查询实际数据
	// 这里是简化实现，实际需要查询 consumption_records 表

	return report, nil
}

// GetModelCostBreakdown 获取模型成本拆解
func GetModelCostBreakdown(userId uint, startDate, endDate time.Time) ([]ModelCostBreakdown, error) {
	// TODO: 实现数据库查询
	breakdown := []ModelCostBreakdown{
		{
			Model:          "claude-sonnet-4-6",
			Requests:        1000,
			InputTokens:     500000,
			OutputTokens:    100000,
			Cost:            decimal.NewFromFloat(15.0),
			PercentOfTotal:  0.6,
		},
		{
			Model:          "gpt-4o",
			Requests:        500,
			InputTokens:     200000,
			OutputTokens:    50000,
			Cost:            decimal.NewFromFloat(7.5),
			PercentOfTotal:  0.3,
		},
		{
			Model:          "gemini-2.5-flash",
			Requests:        200,
			InputTokens:     100000,
			OutputTokens:    20000,
			Cost:            decimal.NewFromFloat(2.5),
			PercentOfTotal:  0.1,
		},
	}
	return breakdown, nil
}

// GetTimeCostBreakdown 获取时段成本拆解
func GetTimeCostBreakdown(userId uint, startDate, endDate time.Time) ([]TimeCostBreakdown, error) {
	// TODO: 实现数据库查询
	breakdown := []TimeCostBreakdown{}
	return breakdown, nil
}

// ExportCostReportCSV 导出成本报告为 CSV
func ExportCostReportCSV(userId uint, req CostReportRequest) ([]byte, error) {
	report, err := GetCostReport(userId, req)
	if err != nil {
		return nil, err
	}

	// 生成 CSV 内容
	var csv string
	csv += "Date,Model,Requests,Input Tokens,Output Tokens,Cost (USD)\n"
	for _, m := range report.ModelBreakdown {
		csv += report.StartDate + "," + m.Model + "," +
			string(rune(m.Requests)) + "," +
			string(rune(m.InputTokens)) + "," +
			string(rune(m.OutputTokens)) + "," +
			m.Cost.String() + "\n"
	}

	return []byte(csv), nil
}
