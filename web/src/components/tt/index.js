/*
TT Components - TokenKey 前端组件统一导出
Copyright (C) 2026 TokenKey
*/

// 用户仪表盘
export { default as UserDashboard } from './dashboard/UserDashboard';

// 团队管理
export { default as TeamManagement } from './team/TeamManagement';

// 管理员控制台
export { default as AdminConsole } from './admin/AdminConsole';

// 导出所有组件
export default {
  UserDashboard: require('./dashboard/UserDashboard').default,
  TeamManagement: require('./team/TeamManagement').default,
  AdminConsole: require('./admin/AdminConsole').default,
};
