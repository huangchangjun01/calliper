import { useEffect } from 'react';
import { Card, Descriptions, Tag, Form, Input, Button, message, Spin } from 'antd';
import { useMyProfile, useUpdateMyProfile, useChangeMyPassword } from '@/services/account';
import { useAuthStore } from '@/stores/authStore';
import dayjs from 'dayjs';

const ROLE_LABELS: Record<string, string> = {
  admin: '管理员',
  user: '普通用户',
  viewer: '只读用户',
};

const ROLE_COLORS: Record<string, string> = {
  admin: 'red',
  user: 'blue',
  viewer: 'default',
};

export default function Account() {
  const { data: profile, isLoading } = useMyProfile();
  const updateProfile = useUpdateMyProfile();
  const changePassword = useChangeMyPassword();
  const setUser = useAuthStore((s) => s.setUser);
  const [profileForm] = Form.useForm();
  const [passwordForm] = Form.useForm();

  // 资料加载完成后回填邮箱
  useEffect(() => {
    if (profile) {
      profileForm.setFieldsValue({ email: profile.email });
    }
  }, [profile, profileForm]);

  const handleUpdateProfile = async () => {
    try {
      const values = await profileForm.validateFields();
      const updated = await updateProfile.mutateAsync({ email: values.email });
      setUser({
        id: profile?.id ?? '',
        username: profile?.username ?? '',
        email: updated.email,
        role: updated.role,
      });
      message.success('资料已更新');
    } catch (err) {
      message.error(err instanceof Error ? err.message : '更新失败');
    }
  };

  const handleChangePassword = async () => {
    try {
      const values = await passwordForm.validateFields();
      await changePassword.mutateAsync({
        old_password: values.oldPassword,
        new_password: values.newPassword,
      });
      message.success('密码修改成功');
      passwordForm.resetFields();
    } catch (err) {
      message.error(err instanceof Error ? err.message : '修改失败');
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <Card title="账号信息" loading={isLoading}>
        {profile && (
          <Descriptions column={2} bordered size="middle">
            <Descriptions.Item label="用户名">{profile.username}</Descriptions.Item>
            <Descriptions.Item label="角色">
              <Tag color={ROLE_COLORS[profile.role]}>
                {ROLE_LABELS[profile.role] ?? profile.role}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="邮箱">{profile.email}</Descriptions.Item>
            <Descriptions.Item label="状态">
              <Tag color={profile.status === 'active' ? 'green' : 'default'}>
                {profile.status === 'active' ? '正常' : '已禁用'}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="注册时间" span={2}>
              {dayjs(profile.createdAt).format('YYYY-MM-DD HH:mm:ss')}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Card>

      <Card title="修改资料">
        <Form form={profileForm} layout="vertical" style={{ maxWidth: 420 }}>
          <Form.Item
            label="邮箱"
            name="email"
            rules={[
              { required: true, message: '请输入邮箱' },
              { type: 'email', message: '请输入有效的邮箱地址' },
            ]}
          >
            <Input placeholder="请输入邮箱" />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              onClick={handleUpdateProfile}
              loading={updateProfile.isPending}
            >
              保存资料
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Card title="修改密码">
        <Form form={passwordForm} layout="vertical" style={{ maxWidth: 420 }}>
          <Form.Item
            label="旧密码"
            name="oldPassword"
            rules={[{ required: true, message: '请输入旧密码' }]}
          >
            <Input.Password placeholder="请输入旧密码" autoComplete="current-password" />
          </Form.Item>
          <Form.Item
            label="新密码"
            name="newPassword"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 6, message: '密码长度不能少于6位' },
            ]}
          >
            <Input.Password placeholder="请输入新密码（至少6位）" autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            label="确认新密码"
            name="confirmPassword"
            dependencies={['newPassword']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('newPassword') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('两次输入的密码不一致'));
                },
              }),
            ]}
          >
            <Input.Password placeholder="请再次输入新密码" autoComplete="new-password" />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              onClick={handleChangePassword}
              loading={changePassword.isPending}
            >
              修改密码
            </Button>
          </Form.Item>
        </Form>
      </Card>

      {isLoading && !profile && (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin />
        </div>
      )}
    </div>
  );
}
