// Package controller 提供TT API控制器
// sso.go - 企业 SSO 控制器
package controller

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// ========== 企业 SSO（V2.0功能） ==========

// GetSSOProviders 获取可用的 SSO 提供商列表
func GetSSOProviders(c *gin.Context) {
	configs, err := ttmodel.GetSSOConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get SSO configs"})
		return
	}

	providers := make([]ttmodel.SSOProviderInfo, 0, len(configs))
	for _, config := range configs {
		providers = append(providers, ttmodel.SSOProviderInfo{
			Id:             config.Id,
			Provider:       config.Provider,
			Name:           config.Name,
			Enabled:        config.Enabled,
			LoginUrl:       getSSOLoginUrl(config),
			AllowedDomains: strings.Split(config.AllowedDomains, ","),
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": providers})
}

// GetSSOConfig 获取 SSO 配置详情
func GetSSOConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}

	config, err := ttmodel.GetSSOConfig(common.String2Int(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO config not found"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// CreateSSOConfigRequest 创建 SSO 配置请求
type CreateSSOConfigRequest struct {
	Provider       ttmodel.SSOProvider `json:"provider" binding:"required"`
	Name           string              `json:"name" binding:"required"`
	ClientId       string              `json:"client_id"`
	ClientSecret   string              `json:"client_secret"`
	IssuerUrl      string              `json:"issuer_url"`
	AuthUrl        string              `json:"auth_url"`
	TokenUrl       string              `json:"token_url"`
	UserInfoUrl    string              `json:"userinfo_url"`
	AllowedDomains string              `json:"allowed_domains"`
	JitCreateUser  bool                `json:"jit_create_user"`
	DefaultGroup   string              `json:"default_group"`
	RedirectUrl    string              `json:"redirect_url"`
}

// CreateSSOConfig 创建 SSO 配置
func CreateSSOConfig(c *gin.Context) {
	var req CreateSSOConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	config := &ttmodel.SSOConfig{
		Provider:        req.Provider,
		Name:            req.Name,
		ClientId:        req.ClientId,
		ClientSecret:    req.ClientSecret,
		IssuerUrl:       req.IssuerUrl,
		AuthUrl:         req.AuthUrl,
		TokenUrl:        req.TokenUrl,
		UserInfoUrl:     req.UserInfoUrl,
		AllowedDomains:  req.AllowedDomains,
		JitCreateUser:   req.JitCreateUser,
		DefaultGroup:    req.DefaultGroup,
		RedirectUrl:     req.RedirectUrl,
		Enabled:         true,
	}

	if err := ttmodel.CreateSSOConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create SSO config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"id":      config.Id,
	})
}

// UpdateSSOConfigRequest 更新 SSO 配置请求
type UpdateSSOConfigRequest struct {
	Id             uint                `json:"id" binding:"required"`
	Name           string              `json:"name"`
	ClientId       string              `json:"client_id"`
	ClientSecret   string              `json:"client_secret"`
	IssuerUrl      string              `json:"issuer_url"`
	AuthUrl        string              `json:"auth_url"`
	TokenUrl       string              `json:"token_url"`
	UserInfoUrl    string              `json:"userinfo_url"`
	AllowedDomains string              `json:"allowed_domains"`
	Enabled        *bool               `json:"enabled"`
	JitCreateUser  *bool               `json:"jit_create_user"`
	RedirectUrl    string              `json:"redirect_url"`
}

// UpdateSSOConfig 更新 SSO 配置
func UpdateSSOConfig(c *gin.Context) {
	var req UpdateSSOConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	config, err := ttmodel.GetSSOConfig(req.Id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SSO config not found"})
		return
	}

	// 更新字段
	if req.Name != "" {
		config.Name = req.Name
	}
	if req.ClientId != "" {
		config.ClientId = req.ClientId
	}
	if req.ClientSecret != "" {
		config.ClientSecret = req.ClientSecret
	}
	if req.IssuerUrl != "" {
		config.IssuerUrl = req.IssuerUrl
	}
	if req.AuthUrl != "" {
		config.AuthUrl = req.AuthUrl
	}
	if req.TokenUrl != "" {
		config.TokenUrl = req.TokenUrl
	}
	if req.UserInfoUrl != "" {
		config.UserInfoUrl = req.UserInfoUrl
	}
	if req.AllowedDomains != "" {
		config.AllowedDomains = req.AllowedDomains
	}
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.JitCreateUser != nil {
		config.JitCreateUser = *req.JitCreateUser
	}
	if req.RedirectUrl != "" {
		config.RedirectUrl = req.RedirectUrl
	}

	if err := ttmodel.UpdateSSOConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update SSO config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteSSOConfig 删除 SSO 配置
func DeleteSSOConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}

	if err := ttmodel.DeleteSSOConfig(common.String2Int(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete SSO config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// SSOLoginRequest SSO 登录请求
type SSOLoginRequest struct {
	Provider string `json:"provider" binding:"required"`
	Redirect string `json:"redirect"`
}

// InitiateSSOLogin 发起 SSO 登录
func InitiateSSOLogin(c *gin.Context) {
	var req SSOLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	config, err := ttmodel.GetSSOConfigByProvider(ttmodel.SSOProvider(req.Provider))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SSO provider not found"})
		return
	}

	// 生成 state
	state := generateState(req.Redirect)

	// 构建 OAuth2 授权 URL
	authUrl := buildOAuth2AuthUrl(config, state)

	c.JSON(http.StatusOK, gin.H{
		"redirect_url": authUrl,
		"state":        state,
	})
}

// OIDCCallbackRequest OIDC 回调请求
type OIDCCallbackRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state"`
}

// HandleOIDCCallback 处理 OIDC 回调
func HandleOIDCCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code required"})
		return
	}

	// TODO: 验证 state
	_ = state

	// TODO: 使用 code 换取 token
	// TODO: 获取用户信息
	// TODO: 创建或更新用户

	// 简化实现：返回模拟响应
	c.JSON(http.StatusOK, ttmodel.SSOLoginResponse{
		Success:   true,
		Token:     "mock_token_" + common.GetRandomString(32),
		UserId:    1,
		Email:     "user@example.com",
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		Message:   "Login successful",
	})
}

// HandleSAMLCallback 处理 SAML 回调
func HandleSAMLCallback(c *gin.Context) {
	samlResponse := c.PostForm("SAMLResponse")
	if samlResponse == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SAMLResponse required"})
		return
	}

	// 解码 SAML Response
	decoded, err := base64.StdEncoding.DecodeString(samlResponse)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid SAML response"})
		return
	}

	logger.LogInfo(c, "[SAML] Received response: %s", string(decoded))

	// TODO: 解析 SAML Response
	// TODO: 验证签名
	// TODO: 提取用户信息

	c.JSON(http.StatusOK, ttmodel.SSOLoginResponse{
		Success: true,
		Message: "SAML login successful",
	})
}

// GetPredefinedOIDCProviders 获取预定义的 OIDC 提供商
func GetPredefinedOIDCProviders(c *gin.Context) {
	providers := make([]map[string]interface{}, 0)
	for key, config := range ttmodel.PredefinedOIDCProviders {
		providers = append(providers, map[string]interface{}{
			"key":         key,
			"name":        config.Name,
			"issuer_url":  config.IssuerUrl,
			"auth_url":    config.AuthUrl,
			"token_url":   config.TokenUrl,
			"userinfo_url": config.UserInfoUrl,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": providers})
}

// 辅助函数

func getSSOLoginUrl(config ttmodel.SSOConfig) string {
	switch config.Provider {
	case ttmodel.SSOProviderOIDC, ttmodel.SSOProviderOAuth2:
		return buildOAuth2AuthUrl(&config, "")
	case ttmodel.SSOProviderSAML:
		return "/saml/sso/" + fmt.Sprintf("%d", config.Id)
	default:
		return ""
	}
}

func buildOAuth2AuthUrl(config *ttmodel.SSOConfig, state string) string {
	params := fmt.Sprintf(
		"?client_id=%s&redirect_uri=%s&response_type=code&scope=openid%%20email%%20profile",
		config.ClientId,
		config.RedirectUrl,
	)
	if state != "" {
		params += "&state=" + state
	}
	return config.AuthUrl + params
}

func generateState(redirect string) string {
	state := map[string]interface{}{
		"redirect": redirect,
		"nonce":    common.GetRandomString(16),
		"timestamp": time.Now().Unix(),
	}
	data, _ := json.Marshal(state)
	return base64.URLEncoding.EncodeToString(data)
}
