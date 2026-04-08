/*
TT Team Management - TokenKey 团队管理
Copyright (C) 2026 TokenKey
*/

import React, { useState, useEffect } from 'react';
import { Card, Row, Col, Typography, Button, Table, Tag, Modal, Form, Input, Select, Avatar, Dropdown, Toast, Spin, Empty, Popconfirm, Descriptions } from '@douyinfe/semi-ui';
import {
  IconUserGroup,
  IconPlus,
  IconMore,
  IconKey,
  IconSetting,
  IconDelete,
  IconCreditCard,
  IconUserAdd,
} from '@douyinfe/semi-icons';
import { API } from '../../../helpers/api';

const { Title, Text, Paragraph } = Typography;

// 角色颜色映射
const ROLE_COLORS = {
  owner: 'red',
  admin: 'orange',
  member: 'blue'
};

const ROLE_LABELS = {
  owner: '所有者',
  admin: '管理员',
  member: '成员'
};

// 团队列表卡片
const TeamListCard = ({ teams, onCreate, onSelect, loading }) => {
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [form, setForm] = useState({ name: '', description: '' });

  const handleCreate = async () => {
    if (!form.name) {
      Toast.error('请输入团队名称');
      return;
    }
    await onCreate(form);
    setCreateModalVisible(false);
    setForm({ name: '', description: '' });
  };

  const columns = [
    {
      title: '团队',
      dataIndex: 'name',
      key: 'name',
      render: (name, record) => (
        <div className="flex items-center gap-3 cursor-pointer" onClick={() => onSelect(record.id)}>
          <Avatar size={40} color={record.avatar_color || 'blue'}>
            {name.charAt(0).toUpperCase()}
          </Avatar>
          <div>
            <Text strong>{name}</Text>
            <Text type="secondary" size="small" className="block">
              {record.description || '暂无描述'}
            </Text>
          </div>
        </div>
      )
    },
    {
      title: '成员数',
      dataIndex: 'member_count',
      key: 'member_count',
      width: 100,
      render: (count) => (
        <Tag color="blue">{count || 1}</Tag>
      )
    },
    {
      title: '余额',
      dataIndex: 'balance',
      key: 'balance',
      width: 120,
      render: (balance) => (
        <Text strong className="tt-mono" style={{ color: 'var(--tt-success)' }}>
          ${parseFloat(balance || 0).toFixed(2)}
        </Text>
      )
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status) => (
        <Tag color={status === 'active' ? 'green' : 'red'}>
          {status === 'active' ? '正常' : '已停用'}
        </Tag>
      )
    },
    {
      title: '操作',
      key: 'actions',
      width: 80,
      render: (_, record) => (
        <Button
          type="tertiary"
          size="small"
          onClick={() => onSelect(record.id)}
        >
          管理
        </Button>
      )
    }
  ];

  return (
    <Card className="tt-card">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <IconUserGroup size={20} style={{ color: 'var(--semi-color-primary)' }} />
          <Text strong>我的团队</Text>
        </div>
        <Button
          type="primary"
          size="small"
          icon={<IconPlus />}
          onClick={() => setCreateModalVisible(true)}
        >
          创建团队
        </Button>
      </div>

      <Spin spinning={loading}>
        {teams.length === 0 ? (
          <Empty description="暂无团队" />
        ) : (
          <Table
            columns={columns}
            dataSource={teams}
            pagination={false}
            size="small"
          />
        )}
      </Spin>

      <Modal
        title="创建团队"
        visible={createModalVisible}
        onOk={handleCreate}
        onCancel={() => setCreateModalVisible(false)}
      >
        <Form>
          <Form.Input
            field="name"
            label="团队名称"
            placeholder="如: 产品开发组"
            value={form.name}
            onChange={(value) => setForm({ ...form, name: value })}
            rules={[{ required: true, message: '请输入团队名称' }]}
          />
          <Form.TextArea
            field="description"
            label="描述"
            placeholder="团队简介（可选）"
            value={form.description}
            onChange={(value) => setForm({ ...form, description: value })}
          />
        </Form>
      </Modal>
    </Card>
  );
};

// 团队详情组件
const TeamDetailCard = ({ team, members, apiKeys, onAddMember, onRemoveMember, onUpdateRole, onCreateKey, onRevokeKey, onBack, loading }) => {
  const [addMemberModalVisible, setAddMemberModalVisible] = useState(false);
  const [createKeyModalVisible, setCreateKeyModalVisible] = useState(false);
  const [addMemberForm, setAddMemberForm] = useState({ userId: '', role: 'member' });
  const [createKeyForm, setCreateKeyForm] = useState({ name: '', description: '' });

  const memberColumns = [
    {
      title: '成员',
      key: 'user',
      render: (_, record) => (
        <div className="flex items-center gap-2">
          <Avatar size={32}>{record.username?.charAt(0)?.toUpperCase() || '?'}</Avatar>
          <div>
            <Text>{record.username || `用户${record.user_id}`}</Text>
          </div>
        </div>
      )
    },
    {
      title: '角色',
      dataIndex: 'role',
      key: 'role',
      render: (role) => (
        <Tag color={ROLE_COLORS[role]}>{ROLE_LABELS[role]}</Tag>
      )
    },
    {
      title: '加入时间',
      dataIndex: 'joined_at',
      key: 'joined_at',
      render: (date) => new Date(date).toLocaleDateString()
    },
    {
      title: '操作',
      key: 'actions',
      render: (_, record) => {
        const isOwner = team?.owner_id === record.user_id;
        if (isOwner) return null;

        return (
          <Dropdown
            content={
              <Dropdown.Menu>
                {record.role !== 'admin' && (
                  <Dropdown.Item onClick={() => onUpdateRole(record.user_id, 'admin')}>
                    <IconUserAdd /> 设为管理员
                  </Dropdown.Item>
                )}
                {record.role !== 'member' && (
                  <Dropdown.Item onClick={() => onUpdateRole(record.user_id, 'member')}>
                    <IconUserAdd /> 设为成员
                  </Dropdown.Item>
                )}
                <Dropdown.Divider />
                <Dropdown.Item type="danger" onClick={() => onRemoveMember(record.user_id)}>
                  <IconDelete /> 移除
                </Dropdown.Item>
              </Dropdown.Menu>
            }
          >
            <Button type="tertiary" icon={<IconMore />} size="small" />
          </Dropdown>
        );
      }
    }
  ];

  const keyColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    {
      title: 'Key',
      dataIndex: 'key',
      key: 'key',
      render: (key) => (
        <Text code>{key.substring(0, 16)}...</Text>
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
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (date) => new Date(date).toLocaleDateString()
    },
    {
      title: '操作',
      key: 'actions',
      render: (_, record) => (
        <Popconfirm
          title="确定吊销此 Key？"
          onConfirm={() => onRevokeKey(record.id)}
        >
          <Button type="danger" size="small" icon={<IconDelete />}>
            吊销
          </Button>
        </Popconfirm>
      )
    }
  ];

  return (
    <div className="space-y-4">
      {/* 返回按钮 */}
      <Button type="tertiary" onClick={onBack}>
        ← 返回团队列表
      </Button>

      {/* 团队信息 */}
      <Card className="tt-card">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <Avatar size={48} color="blue">{team?.name?.charAt(0)?.toUpperCase()}</Avatar>
            <div>
              <Title heading={4}>{team?.name}</Title>
              <Text type="secondary">{team?.description || '暂无描述'}</Text>
            </div>
          </div>
          <Text strong className="tt-balance-amount" style={{ color: 'var(--tt-success)', fontSize: '1.25rem' }}>
            ${parseFloat(team?.balance || 0).toFixed(2)}
          </Text>
        </div>

        <Descriptions>
          <Descriptions.Item label="状态">
            <Tag color={team?.status === 'active' ? 'green' : 'red'}>
              {team?.status === 'active' ? '正常' : '已停用'}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="创建时间">
            {team?.created_at ? new Date(team.created_at).toLocaleDateString() : '-'}
          </Descriptions.Item>
        </Descriptions>
      </Card>

      {/* 成员管理 */}
      <Card className="tt-card">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <IconUserGroup size={18} style={{ color: 'var(--semi-color-primary)' }} />
            <Text strong>成员管理</Text>
          </div>
          <Button
            type="primary"
            size="small"
            icon={<IconUserAdd />}
            onClick={() => setAddMemberModalVisible(true)}
          >
            添加成员
          </Button>
        </div>

        <Table
          columns={memberColumns}
          dataSource={members}
          pagination={false}
          size="small"
        />
      </Card>

      {/* API Key 管理 */}
      <Card className="tt-card">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <IconKey size={18} style={{ color: 'var(--tt-warning)' }} />
            <Text strong>团队 API Keys</Text>
          </div>
          <Button
            type="primary"
            size="small"
            icon={<IconPlus />}
            onClick={() => setCreateKeyModalVisible(true)}
          >
            创建 Key
          </Button>
        </div>

        <Table
          columns={keyColumns}
          dataSource={apiKeys}
          pagination={false}
          size="small"
        />
      </Card>

      {/* 添加成员弹窗 */}
      <Modal
        title="添加成员"
        visible={addMemberModalVisible}
        onOk={async () => {
          await onAddMember(addMemberForm.userId, addMemberForm.role);
          setAddMemberModalVisible(false);
          setAddMemberForm({ userId: '', role: 'member' });
        }}
        onCancel={() => setAddMemberModalVisible(false)}
      >
        <Form>
          <Form.InputNumber
            field="userId"
            label="用户 ID"
            placeholder="输入用户 ID"
            value={addMemberForm.userId}
            onChange={(value) => setAddMemberForm({ ...addMemberForm, userId: value })}
            rules={[{ required: true, message: '请输入用户 ID' }]}
          />
          <Form.Select
            field="role"
            label="角色"
            value={addMemberForm.role}
            onChange={(value) => setAddMemberForm({ ...addMemberForm, role: value })}
          >
            <Select.Option value="admin">管理员</Select.Option>
            <Select.Option value="member">成员</Select.Option>
          </Form.Select>
        </Form>
      </Modal>

      {/* 创建 Key 弹窗 */}
      <Modal
        title="创建团队 API Key"
        visible={createKeyModalVisible}
        onOk={async () => {
          await onCreateKey(createKeyForm.name, createKeyForm.description);
          setCreateKeyModalVisible(false);
          setCreateKeyForm({ name: '', description: '' });
        }}
        onCancel={() => setCreateKeyModalVisible(false)}
      >
        <Form>
          <Form.Input
            field="name"
            label="名称"
            placeholder="如: 生产环境 Key"
            value={createKeyForm.name}
            onChange={(value) => setCreateKeyForm({ ...createKeyForm, name: value })}
            rules={[{ required: true, message: '请输入名称' }]}
          />
          <Form.TextArea
            field="description"
            label="描述"
            placeholder="用途说明（可选）"
            value={createKeyForm.description}
            onChange={(value) => setCreateKeyForm({ ...createKeyForm, description: value })}
          />
        </Form>
      </Modal>
    </div>
  );
};

// 主组件
const TeamManagement = () => {
  const [loading, setLoading] = useState(true);
  const [teams, setTeams] = useState([]);
  const [selectedTeam, setSelectedTeam] = useState(null);
  const [teamDetail, setTeamDetail] = useState(null);
  const [members, setMembers] = useState([]);
  const [apiKeys, setApiKeys] = useState([]);

  const fetchTeams = async () => {
    setLoading(true);
    try {
      const res = await API.get('/tt/teams');
      setTeams(res.data.data || []);
    } catch (error) {
      Toast.error('获取团队列表失败');
    } finally {
      setLoading(false);
    }
  };

  const fetchTeamDetail = async (teamId) => {
    setLoading(true);
    try {
      const [teamRes, membersRes, keysRes] = await Promise.all([
        API.get(`/tt/teams/${teamId}`),
        API.get(`/tt/teams/${teamId}/members`),
        API.get(`/tt/teams/${teamId}/api-keys`)
      ]);

      setTeamDetail(teamRes.data.data);
      setMembers(membersRes.data.data || []);
      setApiKeys(keysRes.data.data || []);
      setSelectedTeam(teamId);
    } catch (error) {
      Toast.error('获取团队详情失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTeams();
  }, []);

  const handleCreateTeam = async (data) => {
    try {
      await API.post('/tt/teams', data);
      Toast.success('团队创建成功');
      fetchTeams();
    } catch (error) {
      Toast.error('创建失败');
    }
  };

  const handleAddMember = async (userId, role) => {
    try {
      await API.post(`/tt/teams/${selectedTeam}/members`, { user_id: userId, role });
      Toast.success('添加成功');
      fetchTeamDetail(selectedTeam);
    } catch (error) {
      Toast.error('添加失败');
    }
  };

  const handleRemoveMember = async (userId) => {
    try {
      await API.delete(`/tt/teams/${selectedTeam}/members/${userId}`);
      Toast.success('已移除');
      fetchTeamDetail(selectedTeam);
    } catch (error) {
      Toast.error('操作失败');
    }
  };

  const handleUpdateRole = async (userId, role) => {
    try {
      await API.put(`/tt/teams/${selectedTeam}/members/${userId}/role`, { role });
      Toast.success('已更新');
      fetchTeamDetail(selectedTeam);
    } catch (error) {
      Toast.error('操作失败');
    }
  };

  const handleCreateKey = async (name, description) => {
    try {
      await API.post(`/tt/teams/${selectedTeam}/api-keys`, { name, description });
      Toast.success('创建成功');
      fetchTeamDetail(selectedTeam);
    } catch (error) {
      Toast.error('创建失败');
    }
  };

  const handleRevokeKey = async (keyId) => {
    try {
      await API.delete(`/tt/teams/${selectedTeam}/api-keys/${keyId}`);
      Toast.success('已吊销');
      fetchTeamDetail(selectedTeam);
    } catch (error) {
      Toast.error('操作失败');
    }
  };

  return (
    <div className="tt-team-management p-4">
      <Title heading={3} className="mb-4">
        团队管理
      </Title>

      {selectedTeam ? (
        <TeamDetailCard
          team={teamDetail}
          members={members}
          apiKeys={apiKeys}
          onAddMember={handleAddMember}
          onRemoveMember={handleRemoveMember}
          onUpdateRole={handleUpdateRole}
          onCreateKey={handleCreateKey}
          onRevokeKey={handleRevokeKey}
          onBack={() => {
            setSelectedTeam(null);
            setTeamDetail(null);
            setMembers([]);
            setApiKeys([]);
          }}
          loading={loading}
        />
      ) : (
        <TeamListCard
          teams={teams}
          onCreate={handleCreateTeam}
          onSelect={fetchTeamDetail}
          loading={loading}
        />
      )}
    </div>
  );
};

export { TeamManagement };
export default TeamManagement;
