// Package model 提供TT数据模型
// sso.go - 企业 SSO 模型
package model

import (
	"time"
)

// SSOProvider SSO 提供商类型
type SSOProvider string

const (
	SSOProviderOIDC   SSOProvider = "oidc"
	SSOProviderSAML   SSOProvider = "saml"
	SSOProviderOAuth2 SSOProvider = "oauth2"
)

// SSOConfig SSO 配置
type SSOConfig struct {
	Id          uint        `json:"id" gorm:"primaryKey"`
	TeamId      *uint       `json:"team_id" gorm:"index"` // 团队级别 SSO

	// 提供商信息
	Provider    SSOProvider `json:"provider" gorm:"size:20;not null"`
	Name        string      `json:"name" gorm:"size:100;not null"`     // 显示名称
	Enabled     bool        `json:"enabled" gorm:"default:false"`

	// OIDC/OAuth2 配置
	ClientId     string `json:"client_id" gorm:"size:255"`
	ClientSecret string `json:"-" gorm:"size:255"` // 不返回给前端
	IssuerUrl    string `json:"issuer_url" gorm:"size:512"`
	AuthUrl      string `json:"auth_url" gorm:"size:512"`
	TokenUrl     string `json:"token_url" gorm:"size:512"`
	UserInfoUrl  string `json:"userinfo_url" gorm:"size:512"`
	JwksUrl      string `json:"jwks_url" gorm:"size:512"`

	// SAML 配置
	EntityId       string `json:"entity_id" gorm:"size:512"`
	SsoUrl         string `json:"sso_url" gorm:"size:512"`
	SloUrl         string `json:"slo_url" gorm:"size:512"`
	Certificate    string `json:"-" gorm:"type:text"` // 不返回给前端
	PrivateKey     string `json:"-" gorm:"type:text"` // 不返回给前端

	// 域名绑定
	AllowedDomains string `json:"allowed_domains" gorm:"size:512"` // 逗号分隔的域名列表

	// 用户映射
	EmailClaim    string `json:"email_claim" gorm:"size:50;default:'email'"`
	NameClaim     string `json:"name_claim" gorm:"size:50;default:'name'"`
	GroupsClaim   string `json:"groups_claim" gorm:"size:50"`

	// JIT 用户创建
	JitCreateUser   bool   `json:"jit_create_user" gorm:"default:true"`
	DefaultGroup    string `json:"default_group" gorm:"size:50"`
	DefaultQuota    int    `json:"default_quota"`

	// 回调配置
	RedirectUrl    string `json:"redirect_url" gorm:"size:512"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (SSOConfig) TableName() string {
	return "sso_configs"
}

// SSOProviderInfo SSO 提供商信息
type SSOProviderInfo struct {
	Id           uint        `json:"id"`
	Provider     SSOProvider `json:"provider"`
	Name         string      `json:"name"`
	Enabled      bool        `json:"enabled"`
	LoginUrl     string      `json:"login_url"`
	AllowedDomains []string   `json:"allowed_domains"`
}

// SSORequest SSO 请求
type SSORequest struct {
	Provider string `json:"provider" binding:"required"`
	TeamId   *uint  `json:"team_id"`
}

// SSOLoginRequest SSO 登录请求
type SSOLoginRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state"`
}

// SSOLoginResponse SSO 登录响应
type SSOLoginResponse struct {
	Success   bool   `json:"success"`
	Token     string `json:"token,omitempty"`
	UserId    uint   `json:"user_id,omitempty"`
	Email     string `json:"email,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
	Message   string `json:"message,omitempty"`
}

// SAMLResponse SAML 响应处理
type SAMLResponse struct {
	NameID      string            `json:"name_id"`
	Email       string            `json:"email"`
	Name        string            `json:"name"`
	Groups      []string          `json:"groups"`
	Attributes  map[string]string `json:"attributes"`
}

// PredefinedOIDCProviders 预定义的 OIDC 提供商
var PredefinedOIDCProviders = map[string]OIDCProviderConfig{
	"google": {
		Name:        "Google",
		IssuerUrl:   "https://accounts.google.com",
		AuthUrl:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenUrl:    "https://oauth2.googleapis.com/token",
		UserInfoUrl: "https://www.googleapis.com/oauth2/v3/userinfo",
		JwksUrl:     "https://www.googleapis.com/oauth2/v3/certs",
	},
	"okta": {
		Name:        "Okta",
		IssuerUrl:   "https://{tenant}.okta.com",
		AuthUrl:     "https://{tenant}.okta.com/oauth2/v1/authorize",
		TokenUrl:    "https://{tenant}.okta.com/oauth2/v1/token",
		UserInfoUrl: "https://{tenant}.okta.com/oauth2/v1/userinfo",
		JwksUrl:     "https://{tenant}.okta.com/oauth2/v1/keys",
	},
	"azure": {
		Name:        "Azure AD",
		IssuerUrl:   "https://login.microsoftonline.com/{tenant}/v2.0",
		AuthUrl:     "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/authorize",
		TokenUrl:    "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token",
		UserInfoUrl: "https://graph.microsoft.com/oidc/userinfo",
		JwksUrl:     "https://login.microsoftonline.com/{tenant}/discovery/v2.0/keys",
	},
	"gitlab": {
		Name:        "GitLab",
		IssuerUrl:   "https://gitlab.com",
		AuthUrl:     "https://gitlab.com/oauth/authorize",
		TokenUrl:    "https://gitlab.com/oauth/token",
		UserInfoUrl: "https://gitlab.com/api/v4/user",
		JwksUrl:     "",
	},
}

// OIDCProviderConfig OIDC 提供商配置
type OIDCProviderConfig struct {
	Name        string `json:"name"`
	IssuerUrl   string `json:"issuer_url"`
	AuthUrl     string `json:"auth_url"`
	TokenUrl    string `json:"token_url"`
	UserInfoUrl string `json:"userinfo_url"`
	JwksUrl     string `json:"jwks_url"`
}

// GetSSOConfigs 获取所有 SSO 配置
func GetSSOConfigs() ([]SSOConfig, error) {
	var configs []SSOConfig
	if err := DB.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// GetSSOConfig 获取单个 SSO 配置
func GetSSOConfig(id uint) (*SSOConfig, error) {
	var config SSOConfig
	if err := DB.First(&config, id).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// GetSSOConfigByProvider 根据 provider 获取配置
func GetSSOConfigByProvider(provider SSOProvider) (*SSOConfig, error) {
	var config SSOConfig
	if err := DB.Where("provider = ? AND enabled = ?", provider, true).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// CreateSSOConfig 创建 SSO 配置
func CreateSSOConfig(config *SSOConfig) error {
	return DB.Create(config).Error
}

// UpdateSSOConfig 更新 SSO 配置
func UpdateSSOConfig(config *SSOConfig) error {
	return DB.Save(config).Error
}

// DeleteSSOConfig 删除 SSO 配置
func DeleteSSOConfig(id uint) error {
	return DB.Delete(&SSOConfig{}, id).Error
}

// IsEmailAllowedForSSO 检查邮箱是否允许使用 SSO
func (c *SSOConfig) IsEmailAllowedForSSO(email string) bool {
	if c.AllowedDomains == "" {
		return true // 未配置域名限制
	}

	domains := splitString(c.AllowedDomains, ",")
	emailDomain := extractEmailDomain(email)

	for _, domain := range domains {
		if domain == emailDomain {
			return true
		}
	}
	return false
}

// splitString 分割字符串
func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

// extractEmailDomain 提取邮箱域名
func extractEmailDomain(email string) string {
	for i := 0; i < len(email); i++ {
		if email[i] == '@' {
			return email[i+1:]
		}
	}
	return ""
}
