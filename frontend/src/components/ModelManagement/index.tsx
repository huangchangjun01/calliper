import { useState } from 'react';
import { Table, Button, Tag, Modal, InputNumber, Space, message } from 'antd';
import {
  PlayCircleOutlined,
  ExperimentOutlined,
  SettingOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import {
  useModels,
  useTrainModel,
  useEvaluateModel,
  useUpdateModelParams,
  useTriggerPrediction,
  type ModelInfo,
  type ShortModelParams,
  type MediumModelParams,
  type LongModelParams,
} from '@/services/admin';
import dayjs from 'dayjs';

const PERIOD_LABELS: Record<string, string> = {
  short: '短期',
  medium: '中短期',
  long: '长期',
};

const STATUS_LABELS: Record<string, { text: string; color: string }> = {
  idle: { text: '空闲', color: 'default' },
  training: { text: '训练中', color: 'processing' },
  evaluating: { text: '评估中', color: 'processing' },
  ready: { text: '就绪', color: 'green' },
  error: { text: '异常', color: 'red' },
};

export default function ModelManagement() {
  const { data: models = [], isLoading } = useModels();
  const trainMutation = useTrainModel();
  const evaluateMutation = useEvaluateModel();
  const paramsMutation = useUpdateModelParams();
  const predictMutation = useTriggerPrediction();

  const [paramsVisible, setParamsVisible] = useState(false);
  const [editingModel, setEditingModel] = useState<ModelInfo | null>(null);
  const [paramsForm, setParamsForm] = useState<Record<string, number>>({});

  const handleTrain = async (id: string) => {
    try {
      await trainMutation.mutateAsync(id);
      message.success('训练已触发');
    } catch {
      message.error('训练失败');
    }
  };

  const handleEvaluate = async (id: string) => {
    try {
      await evaluateMutation.mutateAsync(id);
      message.success('评估已触发');
    } catch {
      message.error('评估失败');
    }
  };

  const handlePredict = async (id: string) => {
    try {
      await predictMutation.mutateAsync(id);
      message.success('预测任务已触发');
    } catch {
      message.error('触发失败');
    }
  };

  const handleParamsOpen = (model: ModelInfo) => {
    setEditingModel(model);
    const form: Record<string, number> = {};
    for (const [key, value] of Object.entries(model.params)) {
      form[key] = value as number;
    }
    setParamsForm(form);
    setParamsVisible(true);
  };

  const handleParamsSave = async () => {
    if (!editingModel) return;
    try {
      await paramsMutation.mutateAsync({
        id: editingModel.id,
        params: paramsForm,
      });
      message.success('参数已保存');
      setParamsVisible(false);
    } catch {
      message.error('保存失败');
    }
  };

  const renderParamFields = () => {
    if (!editingModel) return null;

    if (editingModel.period === 'short') {
      const p = paramsForm as unknown as ShortModelParams;
      return (
        <>
          <div className="admin-form-item">
            <label>hidden_size</label>
            <InputNumber
              value={p.hidden_size}
              onChange={(v) => setParamsForm({ ...paramsForm, hidden_size: v ?? 128 })}
              min={32}
              max={512}
              style={{ width: '100%' }}
            />
          </div>
          <div className="admin-form-item">
            <label>num_layers</label>
            <InputNumber
              value={p.num_layers}
              onChange={(v) => setParamsForm({ ...paramsForm, num_layers: v ?? 2 })}
              min={1}
              max={8}
              style={{ width: '100%' }}
            />
          </div>
          <div className="admin-form-item">
            <label>dropout</label>
            <InputNumber
              value={p.dropout}
              onChange={(v) => setParamsForm({ ...paramsForm, dropout: v ?? 0.2 })}
              min={0}
              max={0.8}
              step={0.05}
              style={{ width: '100%' }}
            />
          </div>
        </>
      );
    }

    if (editingModel.period === 'medium') {
      const p = paramsForm as unknown as MediumModelParams;
      return (
        <>
          <div className="admin-form-item">
            <label>XGBoost max_depth</label>
            <InputNumber
              value={p.xgb_max_depth}
              onChange={(v) => setParamsForm({ ...paramsForm, xgb_max_depth: v ?? 6 })}
              min={2}
              max={15}
              style={{ width: '100%' }}
            />
          </div>
          <div className="admin-form-item">
            <label>XGBoost learning_rate</label>
            <InputNumber
              value={p.xgb_learning_rate}
              onChange={(v) => setParamsForm({ ...paramsForm, xgb_learning_rate: v ?? 0.05 })}
              min={0.001}
              max={1}
              step={0.01}
              style={{ width: '100%' }}
            />
          </div>
          <div className="admin-form-item">
            <label>LightGBM num_leaves</label>
            <InputNumber
              value={p.lgb_num_leaves}
              onChange={(v) => setParamsForm({ ...paramsForm, lgb_num_leaves: v ?? 31 })}
              min={7}
              max={255}
              style={{ width: '100%' }}
            />
          </div>
        </>
      );
    }

    const p = paramsForm as unknown as LongModelParams;
    return (
      <>
        <div className="admin-form-item">
          <label>d_model</label>
          <InputNumber
            value={p.d_model}
            onChange={(v) => setParamsForm({ ...paramsForm, d_model: v ?? 256 })}
            min={64}
            max={1024}
            step={64}
            style={{ width: '100%' }}
          />
        </div>
        <div className="admin-form-item">
          <label>nhead</label>
          <InputNumber
            value={p.nhead}
            onChange={(v) => setParamsForm({ ...paramsForm, nhead: v ?? 8 })}
            min={1}
            max={32}
            style={{ width: '100%' }}
          />
        </div>
        <div className="admin-form-item">
          <label>num_layers</label>
          <InputNumber
            value={p.num_layers}
            onChange={(v) => setParamsForm({ ...paramsForm, num_layers: v ?? 4 })}
            min={1}
            max={12}
            style={{ width: '100%' }}
          />
        </div>
      </>
    );
  };

  const columns: ColumnsType<ModelInfo> = [
    {
      title: '模型名称',
      dataIndex: 'name',
      key: 'name',
      width: 180,
    },
    {
      title: '周期',
      dataIndex: 'period',
      key: 'period',
      width: 80,
      render: (p: string) => <Tag>{PERIOD_LABELS[p] || p}</Tag>,
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 100,
    },
    {
      title: '准确率',
      dataIndex: 'accuracy',
      key: 'accuracy',
      width: 100,
      render: (v: number) => `${(v * 100).toFixed(1)}%`,
    },
    {
      title: '最后训练',
      dataIndex: 'lastTrainTime',
      key: 'lastTrainTime',
      width: 160,
      render: (ts: string | null) =>
        ts ? dayjs(ts).format('YYYY-MM-DD HH:mm') : '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (s: string) => {
        const info = STATUS_LABELS[s] || { text: s, color: 'default' };
        return <Tag color={info.color}>{info.text}</Tag>;
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 320,
      render: (_: unknown, record: ModelInfo) => (
        <Space wrap>
          <Button
            type="link"
            icon={<PlayCircleOutlined />}
            onClick={() => handleTrain(record.id)}
            loading={trainMutation.isPending}
          >
            训练
          </Button>
          <Button
            type="link"
            icon={<ExperimentOutlined />}
            onClick={() => handleEvaluate(record.id)}
            loading={evaluateMutation.isPending}
          >
            评估
          </Button>
          <Button
            type="link"
            icon={<SettingOutlined />}
            onClick={() => handleParamsOpen(record)}
          >
            参数
          </Button>
          <Button
            type="link"
            icon={<ThunderboltOutlined />}
            onClick={() => handlePredict(record.id)}
            loading={predictMutation.isPending}
          >
            预测
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div className="admin-section">
      <div className="admin-section-header">
        <h3>模型管理</h3>
      </div>
      <Table
        columns={columns}
        dataSource={models}
        rowKey="id"
        loading={isLoading}
        pagination={false}
        size="middle"
      />

      {/* 参数配置弹窗 */}
      <Modal
        title={`${editingModel?.name || ''} - 参数配置`}
        open={paramsVisible}
        onOk={handleParamsSave}
        onCancel={() => setParamsVisible(false)}
        confirmLoading={paramsMutation.isPending}
        destroyOnClose
        width={480}
      >
        <div className="admin-form">{renderParamFields()}</div>
      </Modal>
    </div>
  );
}