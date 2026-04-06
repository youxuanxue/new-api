package router

import (
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/tt/controller"

	"github.com/gin-gonic/gin"
)

// SetTTApiRouter 设置TokenKey特有API路由
// 这些是TT产品特有的功能，如余额查询、用量统计、邀请裂变等
func SetTTApiRouter(router *gin.Engine) {
	// TT API路由组
	ttRouter := router.Group("/tt")
	ttRouter.Use(middleware.RouteTag("tt"))
	ttRouter.Use(middleware.TokenAuth())
	{
		// 余额查询
		ttRouter.GET("/balance", controller.GetBalance)

		// 用量统计
		ttRouter.GET("/usage", controller.GetUsage)
		ttRouter.GET("/usage/details", controller.GetUsageDetails)

		// 模型真伪验证
		ttRouter.POST("/verify", controller.VerifyModel)

		// 服务状态
		ttRouter.GET("/status", controller.GetServiceStatus)
	}

	// 邀请裂变路由（V1.0功能）
	referralRouter := router.Group("/tt/referral")
	referralRouter.Use(middleware.RouteTag("tt"))
	referralRouter.Use(middleware.TokenAuth())
	{
		referralRouter.GET("", controller.GetReferralInfo)
		referralRouter.POST("/apply", controller.ApplyReferralCode)
		referralRouter.GET("/records", controller.GetReferralRecords)
	}

	// 订阅路由（V1.0功能）
	subscriptionRouter := router.Group("/tt/subscription")
	subscriptionRouter.Use(middleware.RouteTag("tt"))
	subscriptionRouter.Use(middleware.TokenAuth())
	{
		subscriptionRouter.GET("", controller.GetSubscription)
		subscriptionRouter.POST("/subscribe", controller.Subscribe)
		subscriptionRouter.POST("/cancel", controller.CancelSubscription)
		subscriptionRouter.GET("/plans", controller.ListPlans)
	}

	// 团队工作空间路由（V2.0功能）
	teamRouter := router.Group("/tt/teams")
	teamRouter.Use(middleware.RouteTag("tt"))
	teamRouter.Use(middleware.TokenAuth())
	{
		teamRouter.GET("", controller.ListTeams)
		teamRouter.POST("", controller.CreateTeam)
		teamRouter.GET("/:id", controller.GetTeam)
		teamRouter.POST("/:id/members", controller.AddTeamMember)
		teamRouter.DELETE("/:id/members/:user_id", controller.RemoveTeamMember)
		teamRouter.PUT("/:id/members/:user_id/role", controller.UpdateMemberRole)
		teamRouter.GET("/:id/api-keys", controller.ListTeamAPIKeys)
		teamRouter.POST("/:id/api-keys", controller.CreateTeamAPIKey)
		teamRouter.DELETE("/:id/api-keys/:key_id", controller.RevokeTeamAPIKey)
	}

	// 预算管理路由（V1.0功能）
	budgetRouter := router.Group("/tt/budget")
	budgetRouter.Use(middleware.RouteTag("tt"))
	budgetRouter.Use(middleware.TokenAuth())
	{
		budgetRouter.GET("", controller.GetBudgetConfig)
		budgetRouter.PUT("", controller.SetBudgetConfig)
		budgetRouter.GET("/status", controller.GetBudgetStatus)
	}

	// 调用日志路由（V1.0功能）
	logRouter := router.Group("/tt/logs")
	logRouter.Use(middleware.RouteTag("tt"))
	logRouter.Use(middleware.TokenAuth())
	{
		logRouter.GET("", controller.GetCallLogs)
		logRouter.GET("/:id", controller.GetCallLogDetail)
	}

	// 成本分析报告路由（V2.0功能）
	reportRouter := router.Group("/tt/reports")
	reportRouter.Use(middleware.RouteTag("tt"))
	reportRouter.Use(middleware.TokenAuth())
	{
		reportRouter.GET("/cost", controller.GetCostReport)
		reportRouter.GET("/cost/export", controller.ExportCostReport)
		reportRouter.GET("/breakdown/models", controller.GetModelCostBreakdown)
	}

	// 模型 Playground 路由（V2.0功能）
	playgroundRouter := router.Group("/tt/playground")
	playgroundRouter.Use(middleware.RouteTag("tt"))
	playgroundRouter.Use(middleware.TokenAuth())
	{
		playgroundRouter.GET("/models", controller.GetPlaygroundModels)
		playgroundRouter.POST("/run", controller.RunPlayground)
		playgroundRouter.POST("/run/single", controller.RunPlaygroundSingle)
		playgroundRouter.POST("/run/stream", controller.RunPlaygroundStream)
		playgroundRouter.GET("/history", controller.GetPlaygroundHistory)
	}

	// 企业 SSO 路由（V2.0功能）
	ssoRouter := router.Group("/tt/sso")
	ssoRouter.Use(middleware.RouteTag("tt"))
	{
		// 公开路由
		ssoRouter.GET("/providers", controller.GetSSOProviders)
		ssoRouter.POST("/login", controller.InitiateSSOLogin)
		ssoRouter.GET("/callback/oidc", controller.HandleOIDCCallback)
		ssoRouter.POST("/callback/saml", controller.HandleSAMLCallback)
		ssoRouter.GET("/predefined", controller.GetPredefinedOIDCProviders)
	}
	// SSO 管理路由（需要管理员权限）
	ssoAdminRouter := router.Group("/tt/sso/admin")
	ssoAdminRouter.Use(middleware.RouteTag("tt"))
	ssoAdminRouter.Use(middleware.TokenAuth())
	{
		ssoAdminRouter.GET("/:id", controller.GetSSOConfig)
		ssoAdminRouter.POST("", controller.CreateSSOConfig)
		ssoAdminRouter.PUT("", controller.UpdateSSOConfig)
		ssoAdminRouter.DELETE("/:id", controller.DeleteSSOConfig)
	}

	// 智能路由（V1.0功能）
	router.GET("/tt/router/config", middleware.RouteTag("tt"), middleware.TokenAuth(), controller.GetSmartRouterConfig)
	router.POST("/tt/router/recommend", middleware.RouteTag("tt"), middleware.TokenAuth(), controller.SmartRoute)

	// 语义缓存管理（V2.0功能）
	cacheRouter := router.Group("/tt/cache")
	cacheRouter.Use(middleware.RouteTag("tt"))
	cacheRouter.Use(middleware.TokenAuth())
	{
		cacheRouter.GET("/stats", middleware.GetCacheStatsAPI)
		cacheRouter.POST("/clear", middleware.ClearCacheAPI)
	}

	// SLA 保障路由（V2.0功能）
	slaRouter := router.Group("/tt/sla")
	slaRouter.Use(middleware.RouteTag("tt"))
	slaRouter.Use(middleware.TokenAuth())
	{
		slaRouter.GET("/status", controller.GetSLAStatus)
		slaRouter.GET("/reports", controller.GetSLAReports)
		slaRouter.GET("/reports/:id", controller.GetSLAReportDetail)
		slaRouter.GET("/breaches", controller.GetSLABreaches)
		slaRouter.GET("/incidents", controller.GetSLAIncidents)
		slaRouter.GET("/config", controller.GetSLAConfigAPI)
		slaRouter.PUT("/config", controller.UpdateSLAConfigAPI)
		slaRouter.GET("/tiers", controller.GetSLATiers)
	}
}

// SetTTAdminRouter 设置TokenKey管理后台路由
// 管理后台有独立的权限体系和IP白名单
func SetTTAdminRouter(router *gin.Engine) {
	// 管理后台路由组
	adminRouter := router.Group("/admin")
	adminRouter.Use(middleware.RouteTag("admin"))
	adminRouter.Use(middleware.AdminAuth())
	adminRouter.Use(middleware.AdminIsolation())
	{
		// 运营看板
		adminRouter.GET("/dashboard", controller.GetAdminDashboard)

		// 用户管理
		adminRouter.GET("/users", controller.ListUsers)
		adminRouter.GET("/users/:id", controller.GetUser)
		adminRouter.PUT("/users/:id", controller.UpdateUser)
		adminRouter.POST("/users/:id/adjust-balance", controller.AdjustUserBalance)
		adminRouter.POST("/users/:id/status", controller.SetUserStatus)

		// 渠道管理
		adminRouter.GET("/channels", controller.ListChannels)
		adminRouter.POST("/channels", controller.CreateChannel)
		adminRouter.PUT("/channels/:id", controller.UpdateChannel)
		adminRouter.DELETE("/channels/:id", controller.DeleteChannel)
		adminRouter.POST("/channels/:id/test", controller.TestChannel)

		// 号池管理
		adminRouter.GET("/pool", controller.GetPoolStatus)
		adminRouter.GET("/pool/accounts", controller.ListPoolAccounts)
		adminRouter.POST("/pool/accounts", controller.AddPoolAccount)
		adminRouter.DELETE("/pool/accounts/:id", controller.RemovePoolAccount)
		adminRouter.POST("/pool/accounts/:id/refresh", controller.RefreshPoolAccount)

		// 定价管理
		adminRouter.GET("/pricing", controller.ListPricing)
		adminRouter.POST("/pricing", controller.CreatePricing)
		adminRouter.PUT("/pricing/:id", controller.UpdatePricing)

		// 套餐管理
		adminRouter.GET("/plans", controller.ListAdminPlans)
		adminRouter.POST("/plans", controller.CreatePlan)
		adminRouter.PUT("/plans/:id", controller.UpdatePlan)

		// 财务中心
		adminRouter.GET("/finance/overview", controller.GetFinanceOverview)
		adminRouter.GET("/finance/revenue", controller.GetRevenueReport)
		adminRouter.GET("/finance/costs", controller.GetAdminCostReport)
		adminRouter.GET("/finance/payments", controller.ListPayments)

		// 审计日志
		adminRouter.GET("/audit", controller.ListAuditLogs)

		// 系统设置
		adminRouter.GET("/settings", controller.GetSettings)
		adminRouter.PUT("/settings", controller.UpdateSettings)

		// Webhook管理
		adminRouter.GET("/webhooks", controller.ListWebhooks)
		adminRouter.POST("/webhooks", controller.CreateWebhook)
		adminRouter.PUT("/webhooks/:id", controller.UpdateWebhook)
		adminRouter.DELETE("/webhooks/:id", controller.DeleteWebhook)
		adminRouter.POST("/webhooks/:id/test", controller.TestWebhook)
	}
}

// SetTTPublicRouter 设置公开路由（无需认证）
func SetTTPublicRouter(router *gin.Engine) {
	publicRouter := router.Group("/tt/public")
	publicRouter.Use(middleware.RouteTag("public"))
	{
		// 公开状态页数据
		publicRouter.GET("/status", controller.GetPublicStatus)

		// 公开运行数据
		publicRouter.GET("/stats", controller.GetPublicStats)
	}
}
