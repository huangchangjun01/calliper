import { useState } from 'react';
import { Table, Button, Switch, Tag, Select, Modal, Input, Space, message } from 'antd';
import {
  SettingOutlined,
  SyncOutlined,
  CheckCircleFilled,
  CloseCircleFilled,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import {
  useDataSources,
  useUpdateDataSource,
  useTriggerSync,
  useToggleDataSource,
  type DataSource,
} from '@/services/admin';
import dayjs from 'dayjs';

const FREQUENCY_OPTIONS = [
  { value: '1m', label: '1分钟' },
  { value: '5m', label: '5分钟' },
  { value: '15m', label: '15分钟' },
  { value: '30m', label: '30分钟' },
];

export default function DataSourceConfig() {
  const { data: sources = [], isLoading } = useDataSources();
  const updateMutation = useUpdateDataSource();
  const syncMutation = useTriggerSync();
  const toggleMutation = useToggleDataSource();

  const [editVisible, setEditVisible] = useState(false);
  const [editingSource, setEditingSource] = useState<DataSource | null>(null);
  const [editApiKey, setEditApiKey] = useState('');
  const [editFrequency, setEditFrequency] = useState('1m');

  const handleEdit = (source: DataSource) => {
    setEditingSource(source);
    setEditApiKey(source.apiKey || '');
    setEditFrequency(source.collectFrequency);
    setEditVisible(true);
  };

  const handleSave = async () => {
    if (!editingSource) return;
    try {
      await updateMutation.mutateAsync({
        id: editingSource.id,
        config: {
          apiKey: editApiKey,
          collectFrequency: editFrequency,
          enabled: editingSource.enabled,
        },
      });
      message.success('配置已保存');
      setEditVisible(false);
    } catch {
      message.error('保存失败');
    }
  };

  const handleSync = async (id: string) => {
    try {
      await syncMutation.mutateAsync(id);
      message.success('同步已触发');
    } catch {
      message.error('同步失败');
    }
  };

  const handleToggle = async (id: string, enabled: boolean) => {
    try {
      await toggleMutation.mutateAsync({ id, enabled });
      message.success(enabled ? '已启用' : '已禁用');
    } catch {
      message.error('操作失败');
    }
  };

  const columns: ColumnsType<DataSource> = [
    {
      title: '数据源名称',
      dataIndex: 'name',
      key: 'name',
      width: 180,
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 120,
      render: (type: string) => {
        const labels: Record<string, string> = { rest: 'REST API', websocket: 'WebSocket', grpc: 'gRPC' };
        return <Tag>{labels[type] || type}</Tag>;
      },
    },
    {
      title: '状态',
      key: 'health',
      width: 100,
      render: (_: unknown, record: DataSource) => (
        <Space>
          {record.healthy ? (
            <CheckCircleFilled style={{ color: 'var(--color-success)' }} />
          ) : (
            <CloseCircleFilled style={{ color: 'var(--color-error)' }} />
          )}
          <span>{record.healthy ? '健康' : '异常'}</span>
        </Space>
      ),
    },
    {
      title: '最后同步',
      dataIndex: 'lastSyncTime',
      key: 'lastSyncTime',
      width: 180,
      render: (time: string | null) =>
        time ? dayjs(time).format('YYYY-MM-DD HH:mm:ss') : '-',
    },
    {
      title: '启用',
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled: boolean, record: DataSource) => (
        <Switch
          checked={enabled}
          onChange={(val) => handleToggle(record.id, val)}
          loading={toggleMutation.isPending}
        />
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 200,
      render: (_: unknown, record: DataSource) => (
        <Space>
          <Button
            type="link"
            icon={<SettingOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Button
            type="link"
            icon={<SyncOutlined />}
            loading={syncMutation.isPending}
            onClick={() => handleSync(record.id)}
            disabled={!record.enabled}
          >
            同步
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div className="admin-section">
      <div className="admin-section-header">
        <h3>数据源配置</h3>
      </div>
      <Table
        columns={columns}
        dataSource={sources}
        rowKey="id"
        loading={isLoading}
        pagination={false}
        size="middle"
      />

      <Modal
        title="编辑数据源配置"
        open={editVisible}
        onOk={handleSave}
        onCancel={() => setEditVisible(false)}
        confirmLoading={updateMutation.isPending}
        destroyOnClose
      >
        <div className="admin-form">
          <div className="admin-form-item">
            <label>API Key</label>
            <Input
              value={editApiKey}
              onChange={(e) => setEditApiKey(e.target.value)}
              placeholder="输入 API Key"
            />
          </div>
          <div className="admin-form-item">
            <label>采集频率</label>
            <Select
              value={editFrequency}
              onChange={setEditFrequency}
              options={FREQUENCY_OPTIONS}
              style={{ width: '100%' }}
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}