import { useState } from 'react';
import { Table, Button, Tag, Select, Modal, Input, Space, message, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import {
  useAdminUsers,
  useCreateUser,
  useUpdateUser,
  useDeleteUser,
  type AdminUser,
} from '@/services/admin';
import dayjs from 'dayjs';

const ROLE_OPTIONS = [
  { value: 'admin', label: '管理员' },
  { value: 'user', label: '普通用户' },
  { value: 'viewer', label: '只读用户' },
];

export default function UserManagement() {
  const { data: users = [], isLoading } = useAdminUsers();
  const createMutation = useCreateUser();
  const updateMutation = useUpdateUser();
  const deleteMutation = useDeleteUser();

  const [addVisible, setAddVisible] = useState(false);
  const [editVisible, setEditVisible] = useState(false);
  const [editingUser, setEditingUser] = useState<AdminUser | null>(null);

  // 添加用户表单
  const [addUsername, setAddUsername] = useState('');
  const [addEmail, setAddEmail] = useState('');
  const [addPassword, setAddPassword] = useState('');
  const [addRole, setAddRole] = useState('user');

  // 编辑用户表单
  const [editRole, setEditRole] = useState('user');

  const handleAdd = async () => {
    if (!addUsername || !addEmail || !addPassword) {
      message.warning('请填写完整信息');
      return;
    }
    try {
      await createMutation.mutateAsync({
        username: addUsername,
        email: addEmail,
        password: addPassword,
        role: addRole,
      });
      message.success('用户创建成功');
      setAddVisible(false);
      resetAddForm();
    } catch {
      message.error('创建失败');
    }
  };

  const resetAddForm = () => {
    setAddUsername('');
    setAddEmail('');
    setAddPassword('');
    setAddRole('user');
  };

  const handleEditOpen = (user: AdminUser) => {
    setEditingUser(user);
    setEditRole(user.role);
    setEditVisible(true);
  };

  const handleEditSave = async () => {
    if (!editingUser) return;
    try {
      await updateMutation.mutateAsync({
        id: editingUser.id,
        data: { role: editRole },
      });
      message.success('角色已更新');
      setEditVisible(false);
    } catch {
      message.error('更新失败');
    }
  };

  const handleToggleStatus = async (user: AdminUser) => {
    const newStatus = user.status === 'active' ? 'disabled' : 'active';
    try {
      await updateMutation.mutateAsync({
        id: user.id,
        data: { status: newStatus },
      });
      message.success(newStatus === 'active' ? '已启用' : '已禁用');
    } catch {
      message.error('操作失败');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteMutation.mutateAsync(id);
      message.success('用户已删除');
    } catch {
      message.error('删除失败');
    }
  };

  const columns: ColumnsType<AdminUser> = [
    {
      title: '用户名',
      dataIndex: 'username',
      key: 'username',
      width: 120,
    },
    {
      title: '邮箱',
      dataIndex: 'email',
      key: 'email',
      width: 200,
    },
    {
      title: '角色',
      dataIndex: 'role',
      key: 'role',
      width: 100,
      render: (role: string) => {
        const colors: Record<string, string> = {
          admin: 'red',
          user: 'blue',
          viewer: 'default',
        };
        const labels: Record<string, string> = {
          admin: '管理员',
          user: '普通用户',
          viewer: '只读',
        };
        return <Tag color={colors[role]}>{labels[role]}</Tag>;
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 80,
      render: (status: string) => (
        <Tag color={status === 'active' ? 'green' : 'default'}>
          {status === 'active' ? '启用' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 160,
      render: (ts: string) => dayjs(ts).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '操作',
      key: 'actions',
      width: 200,
      render: (_: unknown, record: AdminUser) => (
        <Space>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEditOpen(record)}
          >
            编辑
          </Button>
          <Button
            type="link"
            onClick={() => handleToggleStatus(record)}
          >
            {record.status === 'active' ? '禁用' : '启用'}
          </Button>
          <Popconfirm
            title="确定删除此用户？"
            onConfirm={() => handleDelete(record.id)}
          >
            <Button type="link" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className="admin-section">
      <div className="admin-section-header">
        <h3>用户管理</h3>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => setAddVisible(true)}
        >
          添加用户
        </Button>
      </div>
      <Table
        columns={columns}
        dataSource={users}
        rowKey="id"
        loading={isLoading}
        pagination={false}
        size="middle"
      />

      {/* 添加用户弹窗 */}
      <Modal
        title="添加用户"
        open={addVisible}
        onOk={handleAdd}
        onCancel={() => {
          setAddVisible(false);
          resetAddForm();
        }}
        confirmLoading={createMutation.isPending}
        destroyOnClose
      >
        <div className="admin-form">
          <div className="admin-form-item">
            <label>用户名</label>
            <Input
              value={addUsername}
              onChange={(e) => setAddUsername(e.target.value)}
              placeholder="输入用户名"
            />
          </div>
          <div className="admin-form-item">
            <label>邮箱</label>
            <Input
              value={addEmail}
              onChange={(e) => setAddEmail(e.target.value)}
              placeholder="输入邮箱"
            />
          </div>
          <div className="admin-form-item">
            <label>密码</label>
            <Input.Password
              value={addPassword}
              onChange={(e) => setAddPassword(e.target.value)}
              placeholder="输入密码"
            />
          </div>
          <div className="admin-form-item">
            <label>角色</label>
            <Select
              value={addRole}
              onChange={setAddRole}
              options={ROLE_OPTIONS}
              style={{ width: '100%' }}
            />
          </div>
        </div>
      </Modal>

      {/* 编辑角色弹窗 */}
      <Modal
        title="编辑用户角色"
        open={editVisible}
        onOk={handleEditSave}
        onCancel={() => setEditVisible(false)}
        confirmLoading={updateMutation.isPending}
        destroyOnClose
      >
        <div className="admin-form">
          <div className="admin-form-item">
            <label>用户名</label>
            <Input value={editingUser?.username} disabled />
          </div>
          <div className="admin-form-item">
            <label>角色</label>
            <Select
              value={editRole}
              onChange={setEditRole}
              options={ROLE_OPTIONS}
              style={{ width: '100%' }}
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}