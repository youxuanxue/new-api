/*
TT User Dashboard - TokenKey 用户仪表盘
Copyright (C) 2026 TokenKey
*/

import React, { useState, useEffect } from 'react';
import { Card, Row, Col, Typography, Progress, Button, Table, Tag, Modal, Form, Input, InputNumber, Select, message, Spin, Empty } from '@douyinfe/semi-ui';
import { IconCreditCard, IconGraph, IconKey, IconUsers, IconTrendingUp, IconAlertTriangle, IconRefresh, IconPlus, IconDelete, IconCopy } from '@douyinfe/semi-icons';
import { API } from '../../helpers/api';

const { Title, Text, Paragraph } = Typography;

// 余额卡片
const BalanceCard = ({ balance, trialBalance, trialUsed, onRecharge }) => {
  const totalBalance = parseFloat(balance || 0) + parseFloat(trialBalance || 0);

  return (
    <Card className="tt-card tt-balance-card">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <IconCreditCard size={20} className="text-blue-500" />
          <Text strong>账户余额</Text>
        </div>
        <Button type="tertiary" size="small" onClick={onRecharge}>
          充值
        </Button>
      </div>

      <Title heading={2} className="text-green-600 mb-2">
        ${totalBalance.toFixed(2)}
      </Title>

      <div className="space-y-1 text-sm text-gray-500">
        <div className="flex justify-between">
          <Text>现金余额</Text>
          <Text strong>${parseFloat(balance || 0).toFixed(2)}</Text>
        </div>
        <div className="flex justify-between">
          <Text>赠送余额</Text>
          <Text strong>${parseFloat(trialBalance || 0).toFixed(2)}</Text>
        </div>
      </div>
    </Card>
  );
};

// 用量统计卡片
const UsageCard = ({ usage }) => {
  const { inputTokens = 0, outputTokens = 0, totalCost = 0 } = usage;

  return (
    <Card className="tt-card tt-usage-card">
      <div className="flex items-center gap-2 mb-4">
        <IconGraph size={20} className="text-purple-500" />
        <Text strong>今日用量</Text>
      </div>

      <Row gutter={16}>
        <Col span={8}>
          <div className="text-center">
            <Text type="secondary" size="small">输入 Token</Text>
            <Title heading={4} className="text-blue-600 mb-0">
              {(inputTokens / 1000).toFixed(1)}K
            </Title>
          </div>
        </Col>
        <Col span={8}>
          <div className="text-center">
            <Text type="secondary" size="small">输出 Token</Text>
            <Title heading={4} className="text-purple-600 mb-0">
              {(outputTokens / 1000).toFixed(1)}K
            </Title>
          </div>
        </Col>
        <Col span={8}>
          <div className="text-center">
            <Text type="secondary" size="small">费用</Text>
            <Title heading={4} className="text-green-600 mb-0">
              ${totalCost.toFixed(4)}
            </Title>
          </div>
        </Col>
      </Row>
    </Card>
  );
};

// API Key 管理卡片
const APIKeyCard = ({ keys, onCreate, onRevoke, loading }) => {
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [form, setForm] = useState({ name: '', limit: 0 });

  const handleCreate = async () => {
    if (!form.name) {
      message.error('请输入 Key 名称');
      return;
    }
    await onCreate(form);
    setCreateModalVisible(false);
    setForm({ name: '', limit: 0 });
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    {
      title: 'Key',
      dataIndex: 'key',
      key: 'key',
      render: (key) => (
        <div className="flex items-center gap-2">
          <Text code>{key.substring(0, 12)}...</Text>
          <Button
            size="small"
            icon={<IconCopy />}
            onClick={() => {
              navigator.clipboard.writeText(key);
              message.success('已复制到剪贴板');
            }}
          />
        </div>
      )
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Tag color={status === 'active' ? 'green' : 'red'}>
          {status === 'active' ? '可用' : '已禁用'}
        </Tag>
      )
    },
    {
      title: '已用额度',
      dataIndex: 'usedQuota',
      key: 'usedQuota',
      render: (used) => `$${parseFloat(used || 0).toFixed(4)}`
    },
    {
      title: '操作',
      key: 'actions',
      render: (_, record) => (
        <Button
          type="danger"
          size="small"
          icon={<IconDelete />}
          onClick={() => onRevoke(record.id)}
        >
          吊销
        </Button>
      )
    }
  ];

  return (
    <Card className="tt-card">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <IconKey size={20} className="text-orange-500" />
          <Text strong>API Keys</Text>
        </div>
        <Button
          type="primary"
          size="small"
          icon={<IconPlus />}
          onClick={() => setCreateModalVisible(true)}
        >
          创建
        </Button>
      </div>

      <Spin spinning={loading}>
        {keys.length === 0 ? (
          <Empty description="暂无 API Key" />
        ) : (
          <Table
            columns={columns}
            dataSource={keys}
            pagination={false}
            size="small"
          />
        )}
      </Spin>

      <Modal
        title="创建 API Key"
        visible={createModalVisible}
        onOk={handleCreate}
        onCancel={() => setCreateModalVisible(false)}
      >
        <Form>
          <Form.Input
            field="name"
            label="名称"
            placeholder="如: 生产环境 Key"
            value={form.name}
            onChange={(value) => setForm({ ...form, name: value })}
          />
          <Form.InputNumber
            field="limit"
            label="额度限制 (USD)"
            placeholder="0 表示无限制"
            value={form.limit}
            onChange={(value) => setForm({ ...form, limit: value })}
            min={0}
          />
        </Form>
      </Modal>
    </Card>
  );
};

// 预算状态卡片
const BudgetCard = ({ budget }) => {
  const { dailyUsed = 0, dailyLimit = 0, monthlyUsed = 0, monthlyLimit = 0, shouldAlert = false } = budget;

  return (
    <Card className="tt-card">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <IconAlertTriangle size={20} className={shouldAlert ? 'text-red-500' : 'text-yellow-500'} />
          <Text strong>预算状态</Text>
        </div>
        {shouldAlert && (
          <Tag color="red" type="solid">需要关注</Tag>
        )}
      </div>

      <div className="space-y-4">
        <div>
          <div className="flex justify-between text-sm mb-1">
            <Text>今日消费</Text>
            <Text>${dailyUsed.toFixed(2)} / ${dailyLimit || '∞'}</Text>
          </div>
          {dailyLimit > 0 && (
            <Progress
              percent={(dailyUsed / dailyLimit) * 100}
              stroke={dailyUsed / dailyLimit > 0.8 ? '#ef4444' : '#10b981'}
              showInfo={false}
            />
          )}
        </div>

        <div>
          <div className="flex justify-between text-sm mb-1">
            <Text>本月消费</Text>
            <Text>${monthlyUsed.toFixed(2)} / ${monthlyLimit || '∞'}</Text>
          </div>
          {monthlyLimit > 0 && (
            <Progress
              percent={(monthlyUsed / monthlyLimit) * 100}
              stroke={monthlyUsed / monthlyLimit > 0.8 ? '#ef4444' : '#3b82f6'}
              showInfo={false}
            />
          )}
        </div>
      </div>
    </Card>
  );
};

// 团队卡片
const TeamCard = ({ teams, onViewTeam }) => {
  return (
    <Card className="tt-card">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <IconUsers size={20} className="text-indigo-500" />
          <Text strong>我的团队</Text>
        </div>
        <Button type="tertiary" size="small" onClick={() => onViewTeam(null)}>
          查看全部
        </Button>
      </div>

      {teams.length === 0 ? (
        <Empty description="暂无团队" />
      ) : (
        <div className="space-y-2">
          {teams.slice(0, 3).map((team) => (
            <div
              key={team.id}
              className="flex items-center justify-between p-2 bg-gray-50 rounded cursor-pointer hover:bg-gray-100"
              onClick={() => onViewTeam(team.id)}
            >
              <div>
                <Text strong>{team.name}</Text>
                <Text type="secondary" size="small" className="block">
                  {team.member_count} 成员
                </Text>
              </div>
              <Tag color="green">${parseFloat(team.balance || 0).toFixed(2)}</Tag>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
};

// 主组件
const UserDashboard = () => {
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [balance, setBalance] = useState({});
  const [usage, setUsage] = useState({});
  const [apiKeys, setApiKeys] = useState([]);
  const [budget, setBudget] = useState({});
  const [teams, setTeams] = useState([]);

  const fetchData = async (showLoading = true) => {
    if (showLoading) setLoading(true);
    try {
      const [balanceRes, usageRes, keysRes, budgetRes, teamsRes] = await Promise.all([
        API.get('/tt/balance'),
        API.get('/tt/usage?period=today'),
        API.get('/api/key'),
        API.get('/tt/budget/status'),
        API.get('/tt/teams')
      ]);

      setBalance(balanceRes.data.data || {});
      setUsage(usageRes.data.data || {});
      setApiKeys(keysRes.data.data || []);
      setBudget(budgetRes.data.data || {});
      setTeams(teamsRes.data.data || []);
    } catch (error) {
      console.error('Failed to fetch dashboard data:', error);
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

  const handleCreateKey = async (data) => {
    try {
      await API.post('/api/key', data);
      message.success('创建成功');
      fetchData(false);
    } catch (error) {
      message.error('创建失败');
    }
  };

  const handleRevokeKey = async (keyId) => {
    try {
      await API.delete(`/api/key/${keyId}`);
      message.success('已吊销');
      fetchData(false);
    } catch (error) {
      message.error('操作失败');
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
    <div className="tt-dashboard p-4">
      <div className="flex items-center justify-between mb-6">
        <Title heading={3}>仪表盘</Title>
        <Button
          type="tertiary"
          icon={<IconRefresh />}
          onClick={handleRefresh}
          loading={refreshing}
        >
          刷新
        </Button>
      </div>

      <Row gutter={[16, 16]}>
        {/* 第一行：余额和用量 */}
        <Col span={12}>
          <BalanceCard
            balance={balance.balance}
            trialBalance={balance.trial_balance}
            trialUsed={balance.trial_used}
            onRecharge={() => window.location.href = '/topup'}
          />
        </Col>
        <Col span={12}>
          <UsageCard usage={usage} />
        </Col>

        {/* 第二行：API Keys */}
        <Col span={24}>
          <APIKeyCard
            keys={apiKeys}
            onCreate={handleCreateKey}
            onRevoke={handleRevokeKey}
            loading={refreshing}
          />
        </Col>

        {/* 第三行：预算和团队 */}
        <Col span={12}>
          <BudgetCard budget={budget} />
        </Col>
        <Col span={12}>
          <TeamCard
            teams={teams}
            onViewTeam={(id) => window.location.href = id ? `/team/${id}` : '/teams'}
          />
        </Col>
      </Row>
    </div>
  );
};

export default UserDashboard;
