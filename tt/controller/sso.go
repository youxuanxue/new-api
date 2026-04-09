// Package controller 提供TT API控制器
// sso.go - 企业 SSO 控制器
package controller

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	ttmodel "github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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

	config, err := ttmodel.GetSSOConfig(uint(common.String2Int(id)))
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

	if err := ttmodel.DeleteSSOConfig(uint(common.String2Int(id))); err != nil {
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

// OIDCTokenResponse OIDC Token 响应
type OIDCTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	IdToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// OIDCUserInfo OIDC 用户信息
type OIDCUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

// HandleOIDCCallback 处理 OIDC 回调
func HandleOIDCCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code required"})
		return
	}

	// 从 state 中恢复 provider 信息
	providerName, redirect, err := parseState(state)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
		return
	}

	// 获取 SSO 配置
	config, err := ttmodel.GetSSOConfigByProvider(ttmodel.SSOProvider(providerName))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SSO provider not found"})
		return
	}

	// 使用 code 换取 token
	tokenResp, err := exchangeCodeForToken(c.Request.Context(), config, code)
	if err != nil {
		logger.LogError(c, "[OIDC] Failed to exchange code: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to exchange code for token"})
		return
	}

	// 获取用户信息
	userInfo, err := fetchOIDCUserInfo(c.Request.Context(), config, tokenResp.AccessToken)
	if err != nil {
		logger.LogError(c, "[OIDC] Failed to fetch user info: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user info"})
		return
	}

	// 检查邮箱域名是否允许
	if !config.IsEmailAllowedForSSO(userInfo.Email) {
		c.JSON(http.StatusForbidden, gin.H{"error": "email domain not allowed"})
		return
	}

	// 创建或更新用户
	user, err := createOrUpdateSSOUser(config, userInfo)
	if err != nil {
		logger.LogError(c, "[OIDC] Failed to create/update user: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create or update user"})
		return
	}

	// 生成 JWT token
	token, expiresAt, err := generateUserToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// 如果有 redirect，重定向到目标页面
	if redirect != "" {
		redirectUrl := fmt.Sprintf("%s?token=%s&expires_at=%d", redirect, token, expiresAt)
		c.Redirect(http.StatusTemporaryRedirect, redirectUrl)
		return
	}

	c.JSON(http.StatusOK, ttmodel.SSOLoginResponse{
		Success:   true,
		Token:     token,
		UserId:    uint(user.Id),
		Email:     userInfo.Email,
		ExpiresAt: expiresAt,
		Message:   "Login successful",
	})
}

// exchangeCodeForToken 使用 authorization code 换取 access token
func exchangeCodeForToken(ctx context.Context, config *ttmodel.SSOConfig, code string) (*OIDCTokenResponse, error) {
	// 构建请求
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", config.RedirectUrl)
	data.Set("client_id", config.ClientId)
	data.Set("client_secret", config.ClientSecret)

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token request returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp OIDCTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// fetchOIDCUserInfo 获取 OIDC 用户信息
func fetchOIDCUserInfo(ctx context.Context, config *ttmodel.SSOConfig, accessToken string) (*OIDCUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", config.UserInfoUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user info request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user info request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user info request returned status %d: %s", resp.StatusCode, string(body))
	}

	var userInfo OIDCUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &userInfo, nil
}

// createOrUpdateSSOUser 创建或更新 SSO 用户
func createOrUpdateSSOUser(config *ttmodel.SSOConfig, userInfo *OIDCUserInfo) (*ttmodel.User, error) {
	// 查找现有用户
	var user ttmodel.User
	result := ttmodel.DB.Where("email = ?", userInfo.Email).First(&user)

	if result.Error != nil {
		// 用户不存在，创建新用户
		if config.JitCreateUser {
			user = ttmodel.User{
				Username:    userInfo.Email,
				Email:       userInfo.Email,
				DisplayName: userInfo.Name,
				Group:       config.DefaultGroup,
				Status:      common.UserStatusEnabled,
			}
			if err := ttmodel.DB.Create(&user).Error; err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}
			ttmodel.RunPostUserCreationHooks(user.Id)
		} else {
			return nil, fmt.Errorf("user not found and JIT creation is disabled")
		}
	} else {
		// 更新现有用户信息
		updates := map[string]interface{}{}
		if userInfo.Name != "" && user.DisplayName != userInfo.Name {
			updates["display_name"] = userInfo.Name
		}
		if len(updates) > 0 {
			if err := ttmodel.DB.Model(&user).Updates(updates).Error; err != nil {
				return nil, fmt.Errorf("failed to update user: %w", err)
			}
		}
	}

	return &user, nil
}

// generateUserToken 生成用户 JWT token
func generateUserToken(user *ttmodel.User) (string, int64, error) {
	expiresAt := time.Now().Add(24 * time.Hour).Unix()

	claims := jwt.MapClaims{
		"user_id": user.Id,
		"email":   user.Email,
		"exp":     expiresAt,
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(common.GetEnvOrDefaultString("JWT_SECRET", "tokenkey-default-secret")))
	if err != nil {
		return "", 0, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
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

	logger.LogInfo(c, fmt.Sprintf("[SAML] Received response: %s", string(decoded)))

	// 解析 SAML Response
	samlResp, err := parseSAMLResponse(decoded)
	if err != nil {
		logger.LogError(c, "[SAML] Failed to parse response: "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse SAML response"})
		return
	}

	// 加载 SAML SSO 配置
	samlConfig, err := ttmodel.GetSSOConfigByProvider(ttmodel.SSOProviderSAML)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "SAML SSO not configured"})
		return
	}

	// 验证 SAML 签名
	if err := verifySAMLSignature(decoded, samlConfig.Certificate); err != nil {
		logger.LogError(c, "[SAML] Signature verification failed: "+err.Error())
		// 在开发环境中，允许跳过签名验证
		if common.GetEnvOrDefaultString("SAML_SKIP_SIGNATURE_CHECK", "false") != "true" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "SAML signature verification failed"})
			return
		}
	}

	// 提取用户信息
	email := samlResp.Email
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no email found in SAML response"})
		return
	}

	// 获取 SSO 配置 (从 SAML Issuer 获取)
	config, err := getSAMLConfigByIssuer(samlResp.Issuer)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SAML issuer not recognized"})
		return
	}

	// 检查邮箱域名
	if !config.IsEmailAllowedForSSO(email) {
		c.JSON(http.StatusForbidden, gin.H{"error": "email domain not allowed"})
		return
	}

	// 创建或更新用户
	userInfo := &OIDCUserInfo{
		Email: email,
		Name:  samlResp.Name,
	}
	user, err := createOrUpdateSSOUser(config, userInfo)
	if err != nil {
		logger.LogError(c, "[SAML] Failed to create/update user: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create or update user"})
		return
	}

	// 生成 JWT token
	token, expiresAt, err := generateUserToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, ttmodel.SSOLoginResponse{
		Success:   true,
		Token:     token,
		UserId:    uint(user.Id),
		Email:     email,
		ExpiresAt: expiresAt,
		Message:   "SAML login successful",
	})
}

// SAMLResponseData SAML 响应数据结构
type SAMLResponseData struct {
	XMLName  xml.Name `xml:"Response"`
	Issuer   string   `xml:"Issuer"`
	NameID   string   `xml:"Assertion>Subject>NameID"`
	Email    string   `xml:"Assertion>AttributeStatement>Attribute>AttributeValue,attr=Name=mail"`
	Name     string   `xml:"Assertion>AttributeStatement>Attribute>AttributeValue,attr=Name=cn"`
	NotOnOrAfter string `xml:"Assertion>Subject>SubjectConfirmation>SubjectConfirmationData,attr=NotOnOrAfter"`
}

// parseSAMLResponse 解析 SAML 响应
func parseSAMLResponse(data []byte) (*SAMLResponseData, error) {
	var resp SAMLResponseData
	if err := xml.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SAML response: %w", err)
	}

	// 尝试从其他属性获取邮箱
	if resp.Email == "" {
		// 某些 IdP 使用 NameID 作为邮箱
		if strings.Contains(resp.NameID, "@") {
			resp.Email = resp.NameID
		}
	}

	return &resp, nil
}

// getSAMLConfigByIssuer 根据 SAML Issuer 获取配置
func getSAMLConfigByIssuer(issuer string) (*ttmodel.SSOConfig, error) {
	var config ttmodel.SSOConfig
	err := ttmodel.DB.Where("provider = ? AND entity_id = ? AND enabled = ?",
		ttmodel.SSOProviderSAML, issuer, true).First(&config).Error
	if err != nil {
		return nil, fmt.Errorf("SAML config not found for issuer: %s", issuer)
	}
	return &config, nil
}

// GetPredefinedOIDCProviders 获取预定义的 OIDC 提供商
func GetPredefinedOIDCProviders(c *gin.Context) {
	providers := make([]map[string]interface{}, 0)
	for key, config := range ttmodel.PredefinedOIDCProviders {
		providers = append(providers, map[string]interface{}{
			"key":          key,
			"name":         config.Name,
			"issuer_url":   config.IssuerUrl,
			"auth_url":     config.AuthUrl,
			"token_url":    config.TokenUrl,
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
		url.QueryEscape(config.RedirectUrl),
	)
	if state != "" {
		params += "&state=" + url.QueryEscape(state)
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

func parseState(state string) (provider string, redirect string, err error) {
	data, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return "", "", err
	}

	var stateData map[string]interface{}
	if err := json.Unmarshal(data, &stateData); err != nil {
		return "", "", err
	}

	if r, ok := stateData["redirect"].(string); ok {
		redirect = r
	}
	return "", redirect, nil
}

// verifySAMLSignature 验证 SAML 签名
func verifySAMLSignature(data []byte, certPEM string) error {
	// 如果证书为空，跳过验证（仅用于开发环境）
	if certPEM == "" {
		return fmt.Errorf("no certificate provided for signature verification")
	}

	// 解析证书
	block, err := pemDecode([]byte(certPEM))
	if err != nil {
		return fmt.Errorf("failed to decode PEM: %w", err)
	}

	cert, err := x509.ParseCertificate(block)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// 提取签名值和签名数据
	signatureValue, signedInfo, err := extractSAMLSignature(data)
	if err != nil {
		return fmt.Errorf("failed to extract signature: %w", err)
	}

	// 如果没有找到签名，返回错误
	if signatureValue == nil {
		return fmt.Errorf("no signature found in SAML response")
	}

	// 计算签名数据的哈希
	hasher := sha256.New()
	hasher.Write(signedInfo)
	hashed := hasher.Sum(nil)

	// 验证签名
	pubKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("certificate does not contain RSA public key")
	}

	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed, signatureValue)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// extractSAMLSignature 从 SAML XML 中提取签名值和签名数据
func extractSAMLSignature(data []byte) ([]byte, []byte, error) {
	// 使用正则表达式提取签名值
	sigValuePattern := regexp.MustCompile(`<ds:SignatureValue[^>]*>([^<]+)</ds:SignatureValue>`)
	sigInfoPattern := regexp.MustCompile(`<ds:SignedInfo[^>]*>([\s\S]*?)</ds:SignedInfo>`)

	// 提取签名值
	sigValueMatch := sigValuePattern.FindSubmatch(data)
	if sigValueMatch == nil {
		return nil, nil, fmt.Errorf("signature value not found")
	}

	sigValue, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigValueMatch[1])))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode signature value: %w", err)
	}

	// 提取签名信息
	sigInfoMatch := sigInfoPattern.FindSubmatch(data)
	if sigInfoMatch == nil {
		return nil, nil, fmt.Errorf("signed info not found")
	}

	// 规范化签名信息（简化实现）
	signedInfo := canonicalizeXML(sigInfoMatch[1])

	return sigValue, signedInfo, nil
}

// canonicalizeXML XML 规范化（简化实现）
func canonicalizeXML(data []byte) []byte {
	// 移除多余的空白字符
	result := strings.ReplaceAll(string(data), "\n", "")
	result = strings.ReplaceAll(result, "\r", "")
	result = strings.ReplaceAll(result, "\t", " ")
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	return []byte(result)
}

// PEM 解码辅助函数 (简化实现)
func pemDecode(data []byte) ([]byte, error) {
	// 简化的 PEM 解码
	str := string(data)
	start := strings.Index(str, "-----BEGIN CERTIFICATE-----")
	end := strings.Index(str, "-----END CERTIFICATE-----")

	if start == -1 || end == -1 {
		return nil, fmt.Errorf("invalid PEM format")
	}

	b64Data := str[start+27 : end]
	return base64.StdEncoding.DecodeString(b64Data)
}
