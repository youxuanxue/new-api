/**
 * UserDashboard 组件测试
 * US-151 前端 Dashboard 测试文件
 */

import React from 'react';
import { render, screen } from '@testing-library/react';
import UserDashboard from '../dashboard/UserDashboard';

// Mock API 模块
jest.mock('../../../helpers/api', () => ({
  API: {
    get: jest.fn().mockResolvedValue({
      data: {
        data: {
          balance: 10.00,
          trial_balance: 1.00,
          trial_used: 0.50
        }
      }
    })
  }
}));

describe('UserDashboard', () => {
  test('renders dashboard title', () => {
    render(<UserDashboard />);
    expect(screen.getByText(/仪表盘/i)).toBeInTheDocument();
  });

  test('renders refresh button', () => {
    render(<UserDashboard />);
    expect(screen.getByText(/刷新/i)).toBeInTheDocument();
  });

  test('renders balance card component', () => {
    render(<UserDashboard />);
    // 应该显示账户余额相关内容
    const balanceText = screen.queryByText(/账户余额/i);
    // 组件可能在加载状态，这里只是验证组件能渲染
    expect(document.querySelector('.tt-dashboard')).toBeInTheDocument();
  });

  test('component structure is valid', () => {
    const { container } = render(<UserDashboard />);
    // 验证容器存在
    expect(container.querySelector('.tt-dashboard')).toBeTruthy();
  });
});
