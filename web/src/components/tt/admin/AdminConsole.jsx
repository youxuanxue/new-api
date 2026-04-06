/*
TT Admin Console - TokenKey 管理员控制台
Copyright (C) 2026 TokenKey
*/

import React, { useState, useEffect } from 'react';
import { Card, Row, Col, Typography, Button, Table, Tag, Modal, Form, Input, InputNumber, Select, Switch, message, Spin, Statistic, Progress, Tabs, TabPane } from '@douyinfe/semi-ui';
import {
  IconDashboard,
  IconUsers,
  IconServer,
  IconKey,
  IconCreditCard,
  IconSettings,
  IconBell,
  IconRefresh,
  IconAlertTriangle,
  IconCheckCircle,
  IconXCircle,
  IconTrendingUp,
  IconTrendingDown,
} from '@douyinfe/semi-icons';
import { API } from '../../helpers/api';

const { Title, Text, Paragraph } = Typography;

// 仪表盘概览卡片
const DashboardOverview = ({ stats }) => {
  const {
    totalUsers = 0,
    activeUsers = 0,
    totalRevenue = 0,
    monthlyRevenue = 0,
    totalRequests = 0,
    avgAvailability = 99.5,
    activeAccounts = 0,
    bannedAccounts = 0
  } = stats;

  return (
    <Row gutter={[16, 16]}>
      <Col span={6}>
        <Card className="tt-stat-card">
          <Statistic
            title="用户总数"
            value={totalUsers}
            suffix={`(${activeUsers} 活跃)`}
            icon={<IconUsers className="text-blue-500" />}
          />
        </Card>
      </Col>
      <Col span={6}>
        <Card className="tt-stat-card">
          <Statistic
            title="本月收入"
            value={monthlyRevenue}
            prefix="$"
            precision={2}
            icon={<IconCreditCard className="text-green-500" />}
          />
        </Card>
      </Col>
      <Col span={6}>
        <Card className="tt-stat-card">
          <Statistic
            title="今日请求"
            value={totalRequests}
            icon={<IconServer className="text-purple-500" />}
          />
        </Card>
      </Col>
      <Col span={6}>
        <Card className="tt-stat-card">
          <Statistic
            title="可用率"
            value={avgAvailability}
            suffix="%"
            icon={<IconCheckCircle className="text-emerald-500" />}
          />
        </Card>
      </Col>
    </Row>
  );
};

// 号池状态卡片
const PoolStatusCard = ({ poolStats, onRefresh }) => {
  const { total = 0, available = 0, cooldown = 0, banned = 0 } = poolStats;
  const utilizationRate = total > 0 ? (available / total) * 100 : 0;

  return (
    <Card
      className="tt-card"
      title={
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <IconServer size={18} className="text-blue-500" />
            <Text strong>号池状态</Text>
          </div>
          <Button type="tertiary" size="small" icon={<IconRefresh />} onClick={onRefresh} />
        </div>
      }
    >
      <Row gutter={16}>
        <Col span={6}>
          <Statistic title="总数" value={total} />
        </Col>
        <Col span={6}>
          <Statistic title="可用" value={available} valueStyle={{ color: '#10b981' }} />
        </Col>
        <Col span={6}>
          <Statistic title="冷却" value={cooldown} valueStyle={{ color: '#f59e0b' }} />
        </Col>
        <Col span={6}>
          <Statistic title="封禁" value={banned} valueStyle={{ color: '#ef4444' }} />
        </Col>
      </Row>

      <div className="mt-4">
        <div className="flex justify-between text-sm mb-1">
          <Text>利用率</Text>
          <Text>{utilizationRate.toFixed(1)}%</Text>
        </div>
        <Progress
          percent={utilizationRate}
          stroke={utilizationRate > 50 ? '#10b981' : '#ef4444'}
          showInfo={false}
        />
      </div>

      {utilizationRate < 30 && (
        <div className="mt-3 p-2 bg-red-50 rounded flex items-center gap-2">
          <IconAlertTriangle className="text-red-500" />
          <Text type="danger" size="small">号池可用率过低，请及时补充</Text>
        </div>
      )}
    </Card>
  );
};

// 用户管理卡片
const UserManagementCard = ({ users, onAdjustBalance, onSetStatus, loading }) => {
  const columns = [
    {
      title: '用户',
      key: 'user',
      render: (_, record) => (
        <div>
          <Text strong>{record.username}</Text>
          <Text type="secondary" size="small" className="block">{record.email}</Text>
        </div>
      )
    },
    {
      title: '余额',
      dataIndex: 'balance',
      key: 'balance',
      render: (balance) => (
        <Text strong className="text-green-600">${parseFloat(balance || 0).toFixed(2)}</Text>
      )
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Tag color={status === 'active' ? 'green' : status === 'suspended' ? 'orange' : 'red'}>
          {status === 'active' ? '正常' : status === 'suspended' ? '暂停' : '封禁'}
        </Tag>
      )
    },
    {
      title: '注册时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (date) => new Date(date).toLocaleDateString()
    },
    {
      title: '操作',
      key: 'actions',
      render: (_, record) => (
        <div className="flex gap-2">
          <Button size="small" onClick={() => onAdjustBalance(record.id)}>
            调整余额
          </Button>
          <Button size="small" type="tertiary" onClick={() => onSetStatus(record.id)}>
            设置状态
          </Button>
        </div>
      )
    }
  ];

  return (
    <Card className="tt-card">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <IconUsers size={18} className="text-blue-500" />
          <Text strong>用户管理</Text>
        </div>
      </div>

      <Spin spinning={loading}>
        <Table
          columns={columns}
          dataSource={users}
          pagination={{ pageSize: 10 }}
          size="small"
        />
      </Spin>
    </Card>
  );
};

// 渠道管理卡片
const ChannelManagementCard = ({ channels, onTest, loading }) => {
  const columns = [
    {
      title: '渠道',
      key: 'channel',
      render: (_, record) => (
        <div>
          <Text strong>{record.name}</Text>
          <Text type="secondary" size="small" className="block">{record.type}</Text>
        </div>
      )
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Tag color={status === 'active' ? 'green' : 'red'}>
          {status === 'active' ? '正常' : '异常'}
        </Tag>
      )
    },
    {
      title: '响应时间',
      dataIndex: 'response_time',
      key: 'response_time',
      render: (time) => time ? `${time}ms` : '-'
    },
    {
      title: '成功率',
      dataIndex: 'success_rate',
      key: 'success_rate',
      render: (rate) => (
        <Progress
          percent={rate || 0}
          size="small"
          stroke={rate > 95 ? '#10b981' : rate > 80 ? '#f59e0b' : '#ef4444'}
          showInfo={false}
          style={{ width: 60 }}
        />
      )
    },
    {
      title: '操作',
      key: 'actions',
      render: (_, record) => (
        <Button size="small" onClick={() => onTest(record.id)}>
          测试
        </Button>
      )
    }
  ];

  return (
    <Card className="tt-card">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <IconServer size={18} className="text-green-500" />
          <Text strong>渠道管理</Text>
        </div>
        <Button type="primary" size="small" icon={<IconPlus />}>
          添加渠道
        </Button>
      </div>

      <Spin spinning={loading}>
        <Table
          columns={columns}
          dataSource={channels}
          pagination={false}
          size="small"
        />
      </Spin>
    </Card>
  );
};

// 审计日志卡片
const AuditLogCard = ({ logs, loading }) => {
  const columns = [
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 150,
      render: (date) => new Date(date).toLocaleString()
    },
    {
      title: '管理员',
      dataIndex: 'admin_name',
      key: 'admin_name',
      width: 100
    },
    {
      title: '操作',
      dataIndex: 'operation',
      key: 'operation',
      width: 150
    },
    {
      title: '目标',
      key: 'target',
      render: (_, record) => (
        <Text type="secondary">{record.target_type}: {record.target_id}</Text>
      )
    },
    {
      title: 'IP',
      dataIndex: 'ip',
      key: 'ip',
      width: 120
    }
  ];

  return (
    <Card className="tt-card">
      <div className="flex items-center gap-2 mb-4">
        <IconBell size={18} className="text-orange-500" />
        <Text strong>审计日志</Text>
      </div>

      <Spin spinning={loading}>
        <Table
          columns={columns}
          dataSource={logs}
          pagination={{ pageSize: 20 }}
          size="small"
        />
      </Spin>
    </Card>
  );
};

// 主组件
const AdminConsole = () => {
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [activeTab, setActiveTab] = useState('dashboard');

  const [stats, setStats] = useState({});
  const [poolStats, setPoolStats] = useState({});
  const [users, setUsers] = useState([]);
  const [channels, setChannels] = useState([]);
  const [logs, setLogs] = useState([]);

  // 调整余额弹窗
  const [adjustBalanceModalVisible, setAdjustBalanceModalVisible] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState(null);
  const [adjustAmount, setAdjustAmount] = useState(0);

  // 设置状态弹窗
  const [setStatusModalVisible, setSetStatusModalVisible] = useState(false);
  const [newStatus, setNewStatus] = useState('active');

  const fetchData = async (showLoading = true) => {
    if (showLoading) setLoading(true);
    try {
      const [dashboardRes, poolRes, usersRes, channelsRes, logsRes] = await Promise.all([
        API.get('/admin/dashboard'),
        API.get('/admin/pool'),
        API.get('/admin/users'),
        API.get('/admin/channels'),
        API.get('/admin/audit')
      ]);

      setStats(dashboardRes.data.data || {});
      setPoolStats(poolRes.data.data || {});
      setUsers(usersRes.data.data || []);
      setChannels(channelsRes.data.data || []);
      setLogs(logsRes.data.data || []);
    } catch (error) {
      console.error('Failed to fetch admin data:', error);
      message.error('获取数据失败');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleRefresh = () => {
    setRefreshing(true);
    fetchData(false);
  };

  const handleAdjustBalance = async () => {
    try {
      await API.post(`/admin/users/${selectedUserId}/adjust-balance`, {
        amount: adjustAmount
      });
      message.success('余额已调整');
      setAdjustBalanceModalVisible(false);
      fetchData(false);
    } catch (error) {
      message.error('调整失败');
    }
  };

  const handleSetStatus = async () => {
    try {
      await API.post(`/admin/users/${selectedUserId}/status`, {
        status: newStatus
      });
      message.success('状态已更新');
      setSetStatusModalVisible(false);
      fetchData(false);
    } catch (error) {
      message.error('操作失败');
    }
  };

  const handleTestChannel = async (channelId) => {
    try {
      const res = await API.post(`/admin/channels/${channelId}/test`);
      if (res.data.success) {
        message.success('渠道正常');
      } else {
        message.error('渠道异常');
      }
    } catch (error) {
      message.error('测试失败');
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div className="tt-admin-console p-4">
      <div className="flex items-center justify-between mb-6">
        <Title heading={3}>管理员控制台</Title>
        <Button
          type="tertiary"
          icon={<IconRefresh />}
          onClick={handleRefresh}
          loading={refreshing}
        >
          刷新
        </Button>
      </div>

      <Tabs activeKey={activeTab} onChange={setActiveTab}>
        <TabPane tab="仪表盘" itemKey="dashboard">
          <div className="space-y-4">
            <DashboardOverview stats={stats} />
            <PoolStatusCard poolStats={poolStats} onRefresh={handleRefresh} />
          </div>
        </TabPane>

        <TabPane tab="用户管理" itemKey="users">
          <UserManagementCard
            users={users}
            onAdjustBalance={(userId) => {
              setSelectedUserId(userId);
              setAdjustBalanceModalVisible(true);
            }}
            onSetStatus={(userId) => {
              setSelectedUserId(userId);
              setSetStatusModalVisible(true);
            }}
            loading={refreshing}
          />
        </TabPane>

        <TabPane tab="渠道管理" itemKey="channels">
          <ChannelManagementCard
            channels={channels}
            onTest={handleTestChannel}
            loading={refreshing}
          />
        </TabPane>

        <TabPane tab="审计日志" itemKey="audit">
          <AuditLogCard logs={logs} loading={refreshing} />
        </TabPane>
      </Tabs>

      {/* 调整余额弹窗 */}
      <Modal
        title="调整用户余额"
        visible={adjustBalanceModalVisible}
        onOk={handleAdjustBalance}
        onCancel={() => setAdjustBalanceModalVisible(false)}
      >
        <Form>
          <Form.InputNumber
            field="amount"
            label="调整金额 (正数为增加，负数为扣除)"
            value={adjustAmount}
            onChange={setAdjustAmount}
            step={1}
          />
        </Form>
      </Modal>

      {/* 设置状态弹窗 */}
      <Modal
        title="设置用户状态"
        visible={setStatusModalVisible}
        onOk={handleSetStatus}
        onCancel={() => setSetStatusModalVisible(false)}
      >
        <Form>
          <Form.Select
            field="status"
            label="用户状态"
            value={newStatus}
            onChange={setNewStatus}
          >
            <Select.Option value="active">正常</Select.Option>
            <Select.Option value="suspended">暂停</Select.Option>
            <Select.Option value="banned">封禁</Select.Option>
          </Form.Select>
        </Form>
      </Modal>
    </div>
  );
};

export default AdminConsole;
