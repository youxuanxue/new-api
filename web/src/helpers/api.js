/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import {
  getUserIdFromLocalStorage,
  showError,
  formatMessageForAPI,
  isValidMessage,
} from './utils';
import axios from 'axios';
import { MESSAGE_ROLES } from '../constants/playground.constants';

export let API = axios.create({
  baseURL: import.meta.env.VITE_REACT_APP_SERVER_URL
    ? import.meta.env.VITE_REACT_APP_SERVER_URL
    : '',
  headers: {
    'New-API-User': getUserIdFromLocalStorage(),
    'Cache-Control': 'no-store',
  },
});


function redirectToOAuthUrl(url, options = {}) {
  const { openInNewTab = false } = options;
  const targetUrl = typeof url === 'string' ? url : url.toString();

  if (openInNewTab) {
    window.open(targetUrl, '_blank');
    return;
  }

  window.location.assign(targetUrl);
}


function patchAPIInstance(instance) {
  const originalGet = instance.get.bind(instance);
  const inFlightGetRequests = new Map();

  const genKey = (url, config = {}) => {
    const params = config.params ? JSON.stringify(config.params) : '{}';
    return `${url}?${params}`;
  };

  instance.get = (url, config = {}) => {
    if (config?.disableDuplicate) {
      return originalGet(url, config);
    }

    const key = genKey(url, config);
    if (inFlightGetRequests.has(key)) {
      return inFlightGetRequests.get(key);
    }

    const reqPromise = originalGet(url, config).finally(() => {
      inFlightGetRequests.delete(key);
    });

    inFlightGetRequests.set(key, reqPromise);
    return reqPromise;
  };
}

patchAPIInstance(API);

export function updateAPI() {
  API = axios.create({
    baseURL: import.meta.env.VITE_REACT_APP_SERVER_URL
      ? import.meta.env.VITE_REACT_APP_SERVER_URL
      : '',
    headers: {
      'New-API-User': getUserIdFromLocalStorage(),
      'Cache-Control': 'no-store',
    },
  });

  patchAPIInstance(API);
}

API.interceptors.response.use(
  (response) => response,
  (error) => {
    // 如果请求配置中显式要求跳过全局错误处理，则不弹出默认错误提示
    if (error.config && error.config.skipErrorHandler) {
      return Promise.reject(error);
    }
    showError(error);
    return Promise.reject(error);
  },
);

// playground

// 构建API请求负载
export const buildApiPayload = (
  messages,
  systemPrompt,
  inputs,
  parameterEnabled,
) => {
  const processedMessages = messages
    .filter(isValidMessage)
    .map(formatMessageForAPI)
    .filter(Boolean);

  // 如果有系统提示，插入到消息开头
  if (systemPrompt && systemPrompt.trim()) {
    processedMessages.unshift({
      role: MESSAGE_ROLES.SYSTEM,
      content: systemPrompt.trim(),
    });
  }

  const payload = {
    model: inputs.model,
    group: inputs.group,
    messages: processedMessages,
    stream: inputs.stream,
  };

  // 添加启用的参数
  const parameterMappings = {
    temperature: 'temperature',
    top_p: 'top_p',
    max_tokens: 'max_tokens',
    frequency_penalty: 'frequency_penalty',
    presence_penalty: 'presence_penalty',
    seed: 'seed',
  };

  Object.entries(parameterMappings).forEach(([key, param]) => {
    const enabled = parameterEnabled[key];
    const value = inputs[param];
    const hasValue = value !== undefined && value !== null;

    if (!enabled) {
      return;
    }

    if (param === 'max_tokens') {
      if (typeof value === 'number') {
        payload[param] = value;
      }
      return;
    }

    if (hasValue) {
      payload[param] = value;
    }
  });

  return payload;
};

// 处理API错误响应
export const handleApiError = (error, response = null) => {
  const errorInfo = {
    error: error.message || '未知错误',
    timestamp: new Date().toISOString(),
    stack: error.stack,
  };

  if (response) {
    errorInfo.status = response.status;
    errorInfo.statusText = response.statusText;
  }

  if (error.message.includes('HTTP error')) {
    errorInfo.details = '服务器返回了错误状态码';
  } else if (error.message.includes('Failed to fetch')) {
    errorInfo.details = '网络连接失败或服务器无响应';
  }

  return errorInfo;
};

// 处理模型数据
export const processModelsData = (data, currentModel) => {
  const modelOptions = data.map((model) => ({
    label: model,
    value: model,
  }));

  const hasCurrentModel = modelOptions.some(
    (option) => option.value === currentModel,
  );
  const selectedModel =
    hasCurrentModel && modelOptions.length > 0
      ? currentModel
      : modelOptions[0]?.value;

  return { modelOptions, selectedModel };
};

// 处理分组数据
export const processGroupsData = (data, userGroup) => {
  let groupOptions = Object.entries(data).map(([group, info]) => ({
    label:
      info.desc.length > 20 ? info.desc.substring(0, 20) + '...' : info.desc,
    value: group,
    ratio: info.ratio,
    fullLabel: info.desc,
  }));

  if (groupOptions.length === 0) {
    groupOptions = [
      {
        label: '用户分组',
        value: '',
        ratio: 1,
      },
    ];
  } else if (userGroup) {
    const userGroupIndex = groupOptions.findIndex((g) => g.value === userGroup);
    if (userGroupIndex > -1) {
      const userGroupOption = groupOptions.splice(userGroupIndex, 1)[0];
      groupOptions.unshift(userGroupOption);
    }
  }

  return groupOptions;
};

// 原来components中的utils.js

export async function getOAuthState() {
  let path = '/api/oauth/state';
  let affCode = localStorage.getItem('aff');
  if (affCode && affCode.length > 0) {
    path += `?aff=${affCode}`;
  }
  const res = await API.get(path);
  const { success, message, data } = res.data;
  if (success) {
    return data;
  } else {
    showError(message);
    return '';
  }
}

async function prepareOAuthState(options = {}) {
  const { shouldLogout = false } = options;
  if (shouldLogout) {
    try {
      await API.get('/api/user/logout', { skipErrorHandler: true });
    } catch (err) {}
    localStorage.removeItem('user');
    updateAPI();
  }
  return await getOAuthState();
}

export async function onDiscordOAuthClicked(client_id, options = {}) {
  const state = await prepareOAuthState(options);
  if (!state) return;
  const redirect_uri = `${window.location.origin}/oauth/discord`;
  const response_type = 'code';
  const scope = 'identify+openid';
  redirectToOAuthUrl(
    `https://discord.com/oauth2/authorize?client_id=${client_id}&redirect_uri=${redirect_uri}&response_type=${response_type}&scope=${scope}&state=${state}`,
  );
}

export async function onOIDCClicked(
  auth_url,
  client_id,
  openInNewTab = false,
  options = {},
) {
  const state = await prepareOAuthState(options);
  if (!state) return;
  const url = new URL(auth_url);
  url.searchParams.set('client_id', client_id);
  url.searchParams.set('redirect_uri', `${window.location.origin}/oauth/oidc`);
  url.searchParams.set('response_type', 'code');
  url.searchParams.set('scope', 'openid profile email');
  url.searchParams.set('state', state);
  redirectToOAuthUrl(url, { openInNewTab });
}

export async function onGitHubOAuthClicked(github_client_id, options = {}) {
  const state = await prepareOAuthState(options);
  if (!state) return;
  redirectToOAuthUrl(
    `https://github.com/login/oauth/authorize?client_id=${github_client_id}&state=${state}&scope=user:email`,
  );
}

export async function onLinuxDOOAuthClicked(
  linuxdo_client_id,
  options = { shouldLogout: false },
) {
  const state = await prepareOAuthState(options);
  if (!state) return;
  redirectToOAuthUrl(
    `https://connect.linux.do/oauth2/authorize?response_type=code&client_id=${linuxdo_client_id}&state=${state}`,
  );
}

/**
 * Initiate custom OAuth login
 * @param {Object} provider - Custom OAuth provider config from status API
 * @param {string} provider.slug - Provider slug (used for callback URL)
 * @param {string} provider.client_id - OAuth client ID
 * @param {string} provider.authorization_endpoint - Authorization URL
 * @param {string} provider.scopes - OAuth scopes (space-separated)
 * @param {Object} options - Options
 * @param {boolean} options.shouldLogout - Whether to logout first
 */
export async function onCustomOAuthClicked(provider, options = {}) {
  const state = await prepareOAuthState(options);
  if (!state) return;

  try {
    const redirect_uri = `${window.location.origin}/oauth/${provider.slug}`;

    // Check if authorization_endpoint is a full URL or relative path
    let authUrl;
    if (
      provider.authorization_endpoint.startsWith('http://') ||
      provider.authorization_endpoint.startsWith('https://')
    ) {
      authUrl = new URL(provider.authorization_endpoint);
    } else {
      // Relative path - this is a configuration error, show error message
      console.error(
        'Custom OAuth authorization_endpoint must be a full URL:',
        provider.authorization_endpoint,
      );
      showError(
        'OAuth 配置错误：授权端点必须是完整的 URL（以 http:// 或 https:// 开头）',
      );
      return;
    }

    authUrl.searchParams.set('client_id', provider.client_id);
    authUrl.searchParams.set('redirect_uri', redirect_uri);
    authUrl.searchParams.set('response_type', 'code');
    authUrl.searchParams.set(
      'scope',
      provider.scopes || 'openid profile email',
    );
    authUrl.searchParams.set('state', state);

    redirectToOAuthUrl(authUrl);
  } catch (error) {
    console.error('Failed to initiate custom OAuth:', error);
    showError('OAuth 登录失败：' + (error.message || '未知错误'));
  }
}

let channelModels = undefined;
export async function loadChannelModels() {
  const res = await API.get('/api/models');
  const { success, data } = res.data;
  if (!success) {
    return;
  }
  channelModels = data;
  localStorage.setItem('channel_models', JSON.stringify(data));
}

export function getChannelModels(type) {
  if (channelModels !== undefined && type in channelModels) {
    if (!channelModels[type]) {
      return [];
    }
    return channelModels[type];
  }
  let models = localStorage.getItem('channel_models');
  if (!models) {
    return [];
  }
  channelModels = JSON.parse(models);
  if (type in channelModels) {
    return channelModels[type];
  }
  return [];
}

// ========== TT API Helper Functions ==========

/**
 * TT User API endpoints
 */
export const TT_API = {
  // Balance & Usage
  getBalance: () => API.get('/tt/balance'),
  getUsage: (period = 'today') => API.get(`/tt/usage?period=${period}`),
  getUsageDetails: (params) => API.get('/tt/usage/details', { params }),

  // Model Verification
  verifyModel: (model) => API.post('/tt/verify', { model }),
  getServiceStatus: () => API.get('/tt/status'),

  // Referral
  getReferralInfo: () => API.get('/tt/referral'),
  applyReferralCode: (code) => API.post('/tt/referral/apply', { invite_code: code }),
  getReferralRecords: () => API.get('/tt/referral/records'),

  // Subscription
  getSubscription: () => API.get('/tt/subscription'),
  subscribePlan: (planId, billingCycle = 'monthly') => API.post('/tt/subscription/subscribe', { plan_id: planId, billing_cycle: billingCycle }),
  cancelSubscription: (reason = '') => API.post('/tt/subscription/cancel', { reason }),
  listPlans: () => API.get('/tt/subscription/plans'),

  // Teams
  listTeams: () => API.get('/tt/teams'),
  createTeam: (name, description = '') => API.post('/tt/teams', { name, description }),
  getTeam: (teamId) => API.get(`/tt/teams/${teamId}`),
  addTeamMember: (teamId, userId, role = 'member') => API.post(`/tt/teams/${teamId}/members`, { user_id: userId, role }),
  removeTeamMember: (teamId, userId) => API.delete(`/tt/teams/${teamId}/members/${userId}`),
  updateMemberRole: (teamId, userId, role) => API.put(`/tt/teams/${teamId}/members/${userId}/role`, { role }),
  listTeamAPIKeys: (teamId) => API.get(`/tt/teams/${teamId}/api-keys`),
  createTeamAPIKey: (teamId, name, description = '') => API.post(`/tt/teams/${teamId}/api-keys`, { name, description }),
  revokeTeamAPIKey: (teamId, keyId) => API.delete(`/tt/teams/${teamId}/api-keys/${keyId}`),

  // Budget
  getBudgetConfig: () => API.get('/tt/budget'),
  setBudgetConfig: (config) => API.put('/tt/budget', config),
  getBudgetStatus: () => API.get('/tt/budget/status'),

  // Logs
  getCallLogs: (params) => API.get('/tt/logs', { params }),
  getCallLogDetail: (logId) => API.get(`/tt/logs/${logId}`),

  // Smart Router
  getSmartRouterConfig: () => API.get('/tt/router/config'),
  getRouteRecommendation: (data) => API.post('/tt/router/recommend', data),

  // Reports (V2.0)
  getCostReport: (params) => API.get('/tt/reports/cost', { params }),
  exportCostReport: (format = 'json') => API.get(`/tt/reports/cost/export?format=${format}`),
  getModelCostBreakdown: (params) => API.get('/tt/reports/breakdown/models', { params }),

  // Playground (V2.0)
  getPlaygroundModels: () => API.get('/tt/playground/models'),
  runPlayground: (data) => API.post('/tt/playground/run', data),
  runPlaygroundSingle: (data) => API.post('/tt/playground/run/single', data),
  getPlaygroundHistory: () => API.get('/tt/playground/history'),

  // SLA (V2.0)
  getSLAStatus: () => API.get('/tt/sla/status'),
  getSLAReports: (params) => API.get('/tt/sla/reports', { params }),
  getSLAReportDetail: (reportId) => API.get(`/tt/sla/reports/${reportId}`),
  getSLABreaches: () => API.get('/tt/sla/breaches'),
  getSLAIncidents: (params) => API.get('/tt/sla/incidents', { params }),
  getSLAConfig: () => API.get('/tt/sla/config'),
  updateSLAConfig: (config) => API.put('/tt/sla/config', config),
  getSLATiers: () => API.get('/tt/sla/tiers'),

  // SSO (V2.0)
  getSSOProviders: () => API.get('/tt/sso/providers'),
  initiateSSOLogin: (provider, redirect) => API.post('/tt/sso/login', { provider, redirect }),
  getPredefinedOIDCProviders: () => API.get('/tt/sso/predefined'),

  // Public
  getPublicStatus: () => API.get('/tt/public/status'),
  getPublicStats: () => API.get('/tt/public/stats'),
};

/**
 * TT Admin API endpoints
 */
export const TT_ADMIN_API = {
  // Dashboard
  getDashboard: () => API.get('/admin/dashboard'),

  // Users
  listUsers: (params) => API.get('/admin/users', { params }),
  getUser: (userId) => API.get(`/admin/users/${userId}`),
  updateUser: (userId, data) => API.put(`/admin/users/${userId}`, data),
  adjustUserBalance: (userId, amount, totpCode = '') => API.post(
    `/admin/users/${userId}/adjust-balance`,
    { amount },
    totpCode ? { headers: { 'X-TOTP-Code': totpCode } } : undefined,
  ),
  setUserStatus: (userId, status, totpCode = '') => API.post(
    `/admin/users/${userId}/status`,
    { status },
    totpCode ? { headers: { 'X-TOTP-Code': totpCode } } : undefined,
  ),

  // Admin Roles
  listAdminRoles: () => API.get('/admin/users/admin-roles'),
  setAdminRole: (userId, role, totpCode = '') => API.post(
    `/admin/users/${userId}/admin-role`,
    { role },
    totpCode ? { headers: { 'X-TOTP-Code': totpCode } } : undefined,
  ),

  // Channels
  listChannels: (params) => API.get('/admin/channels', { params }),
  createChannel: (data) => API.post('/admin/channels', data),
  updateChannel: (channelId, data) => API.put(`/admin/channels/${channelId}`, data),
  deleteChannel: (channelId) => API.delete(`/admin/channels/${channelId}`),
  testChannel: (channelId) => API.post(`/admin/channels/${channelId}/test`),

  // Pool Accounts
  getPoolStatus: () => API.get('/admin/pool'),
  listPoolAccounts: (status) => API.get('/admin/pool/accounts', { params: { status } }),
  addPoolAccount: (data) => API.post('/admin/pool/accounts', data),
  removePoolAccount: (accountId) => API.delete(`/admin/pool/accounts/${accountId}`),
  refreshPoolAccount: (accountId) => API.post(`/admin/pool/accounts/${accountId}/refresh`),

  // Pricing
  listPricing: () => API.get('/admin/pricing'),
  createPricing: (data) => API.post('/admin/pricing', data),
  updatePricing: (pricingId, data) => API.put(`/admin/pricing/${pricingId}`, data),

  // Plans
  listPlans: () => API.get('/admin/plans'),
  createPlan: (data) => API.post('/admin/plans', data),
  updatePlan: (planId, data) => API.put(`/admin/plans/${planId}`, data),

  // Finance
  getFinanceOverview: () => API.get('/admin/finance/overview'),
  getRevenueReport: (params) => API.get('/admin/finance/revenue', { params }),
  getCostReport: (params) => API.get('/admin/finance/costs', { params }),
  listPayments: (params) => API.get('/admin/finance/payments', { params }),

  // Audit
  listAuditLogs: (params) => API.get('/admin/audit', { params }),

  // Settings
  getSettings: () => API.get('/admin/settings'),
  updateSettings: (data) => API.put('/admin/settings', data),

  // Webhooks
  listWebhooks: () => API.get('/admin/webhooks'),
  createWebhook: (data) => API.post('/admin/webhooks', data),
  updateWebhook: (webhookId, data) => API.put(`/admin/webhooks/${webhookId}`, data),
  deleteWebhook: (webhookId) => API.delete(`/admin/webhooks/${webhookId}`),
  testWebhook: (webhookId) => API.post(`/admin/webhooks/${webhookId}/test`),
};
