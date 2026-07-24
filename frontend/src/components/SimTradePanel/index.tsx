import { useState } from 'react';
import { Table, Switch, Tag, Progress, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { PlayCircleOutlined, PauseCircleOutlined } from '@ant-design/icons';
import type { SimStatus, SimDecision, SimRecord } from '@/types';
import dayjs from 'dayjs';

interface SimTradePanelProps {
  simStatus: SimStatus | null;
  loading: boolean;
  onToggle: () => Promise<void>;
}

const SIDE_MAP: Record<string, { label: string; className: string }> = {
  buy: { label: '买入', className: 'trade-buy' },
  sell: { label: '卖出', className: 'trade-sell' },
};

const RISK_STATUS_MAP: Record<string, { label: string; color: string }> = {
  normal: { label: '正常', color: 'success' },
  warning: { label: '预警', color: 'warning' },
  danger: { label: '危险', color: 'error' },
};

export default function SimTradePanel({ simStatus, loading, onToggle }: SimTradePanelProps) {
  const [toggling, setToggling] = useState(false);

  const handleToggle = async () => {
    setToggling(true);
    try {
      await onToggle();
      message.success(simStatus?.running ? '模拟交易已停止' : '模拟交易已启动');
    } catch {
      message.error('操作失败');
    } finally {
      setToggling(false);
    }
  };

  const decisionColumns: ColumnsType<SimDecision> = [
    {
      title: '时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 150,
      render: (val: string) => dayjs(val).format('MM-DD HH:mm:ss'),
    },
    {
      title: '股票',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 110,
      render: (val: string, record) => (
        <span>
          <span className="decision-symbol">{val}</span>
          <span className="decision-name">{record.name}</span>
        </span>
      ),
    },
    {
      title: '方向',
      dataIndex: 'side',
      key: 'side',
      width: 70,
      render: (val: string) => {
        const s = SIDE_MAP[val] || { label: val, className: '' };
        return <span className={s.className}>{s.label}</span>;
      },
    },
    {
      title: '价格',
      dataIndex: 'price',
      key: 'price',
      width: 90,
      align: 'right',
      render: (val: number) => val?.toFixed(2),
    },
    {
      title: '数量',
      dataIndex: 'quantity',
      key: 'quantity',
      width: 80,
      align: 'right',
    },
    {
      title: '置信度',
      dataIndex: 'confidence',
      key: 'confidence',
      width: 100,
      render: (val: number) => (
        <Progress
          percent={Math.round(val * 100)}
          size="small"
          strokeColor={val >= 0.7 ? '#52c41a' : val >= 0.5 ? '#faad14' : '#ff4d4f'}
          format={(p) => `${p}%`}
        />
      ),
    },
    {
      title: '原因',
      dataIndex: 'reason',
      key: 'reason',
      ellipsis: true,
    },
  ];

  const recordColumns: ColumnsType<SimRecord> = [
    {
      title: '时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 150,
      render: (val: string) => dayjs(val).format('MM-DD HH:mm:ss'),
    },
    {
      title: '股票',
      dataIndex: 'symbol',
      key: 'symbol',
      width: 110,
      render: (val: string, record) => (
        <span>
          <span className="decision-symbol">{val}</span>
          <span className="decision-name">{record.name}</span>
        </span>
      ),
    },
    {
      title: '方向',
      dataIndex: 'side',
      key: 'side',
      width: 70,
      render: (val: string) => {
        const s = SIDE_MAP[val] || { label: val, className: '' };
        return <span className={s.className}>{s.label}</span>;
      },
    },
    {
      title: '价格',
      dataIndex: 'price',
      key: 'price',
      width: 90,
      align: 'right',
      render: (val: number) => val?.toFixed(2),
    },
    {
      title: '数量',
      dataIndex: 'quantity',
      key: 'quantity',
      width: 80,
      align: 'right',
    },
    {
      title: '盈亏',
      dataIndex: 'profit',
      key: 'profit',
      width: 100,
      align: 'right',
      render: (val: number) => {
        const className = val > 0 ? 'profit-up' : val < 0 ? 'profit-down' : '';
        return <span className={className}>{val > 0 ? '+' : ''}{val.toFixed(2)}</span>;
      },
    },
    {
      title: '盈亏率',
      dataIndex: 'profitPercent',
      key: 'profitPercent',
      width: 90,
      align: 'right',
      render: (val: number) => {
        const className = val > 0 ? 'profit-up' : val < 0 ? 'profit-down' : '';
        return <span className={className}>{val > 0 ? '+' : ''}{val.toFixed(2)}%</span>;
      },
    },
  ];

  const running = simStatus?.running ?? false;
  const risk = simStatus?.riskControl;

  return (
    <div className="sim-trade-panel">
      {/* 状态开关 */}
      <div className="sim-trade-status">
        <div className="sim-trade-status-left">
          <span className="sim-trade-status-label">模拟交易状态：</span>
          <Tag color={running ? 'success' : 'default'} icon={running ? <PlayCircleOutlined /> : <PauseCircleOutlined />}>
            {running ? '运行中' : '已停止'}
          </Tag>
        </div>
        <div className="sim-trade-status-right">
          <Switch
            checked={running}
            loading={toggling}
            onChange={handleToggle}
            checkedChildren="开"
            unCheckedChildren="关"
          />
        </div>
      </div>

      {/* 模拟账户资产 */}
      {simStatus?.account && (
        <div className="sim-trade-account">
          <div className="sim-trade-account-item">
            <span className="sim-trade-account-label">总资产</span>
            <span className="sim-trade-account-value">¥ {simStatus.account.totalAsset.toFixed(2)}</span>
          </div>
          <div className="sim-trade-account-item">
            <span className="sim-trade-account-label">可用资金</span>
            <span className="sim-trade-account-value">¥ {simStatus.account.availableCash.toFixed(2)}</span>
          </div>
          <div className="sim-trade-account-item">
            <span className="sim-trade-account-label">持仓市值</span>
            <span className="sim-trade-account-value">¥ {simStatus.account.marketValue.toFixed(2)}</span>
          </div>
          <div className="sim-trade-account-item">
            <span className="sim-trade-account-label">今日盈亏</span>
            <span className={`sim-trade-account-value ${simStatus.account.todayProfit >= 0 ? 'profit-up' : 'profit-down'}`}>
              {simStatus.account.todayProfit >= 0 ? '+' : ''}{simStatus.account.todayProfit.toFixed(2)}
            </span>
          </div>
        </div>
      )}

      {/* 风险控制 */}
      {risk && (
        <div className="sim-trade-risk">
          <h4 className="sim-trade-section-title">
            风险控制
            <Tag
              color={RISK_STATUS_MAP[risk.status]?.color}
              style={{ marginLeft: 8 }}
            >
              {RISK_STATUS_MAP[risk.status]?.label}
            </Tag>
          </h4>
          <div className="sim-trade-risk-grid">
            <div className="sim-trade-risk-item">
              <span className="sim-trade-risk-label">单日亏损上限</span>
              <Progress
                percent={Math.min(Math.round((risk.currentDailyLoss / risk.maxDailyLoss) * 100), 100)}
                size="small"
                status={risk.status === 'danger' ? 'exception' : 'active'}
                format={() => `${risk.currentDailyLoss.toFixed(2)} / ${risk.maxDailyLoss.toFixed(2)}`}
              />
            </div>
            <div className="sim-trade-risk-item">
              <span className="sim-trade-risk-label">仓位比例</span>
              <Progress
                percent={Math.min(Math.round(risk.currentPositionRatio * 100), 100)}
                size="small"
                status={risk.status === 'danger' ? 'exception' : 'active'}
                format={() => `${(risk.currentPositionRatio * 100).toFixed(0)}% / ${(risk.maxPositionRatio * 100).toFixed(0)}%`}
              />
            </div>
            <div className="sim-trade-risk-item">
              <span className="sim-trade-risk-label">单只股票上限</span>
              <span className="sim-trade-risk-value">{(risk.maxSingleStockRatio * 100).toFixed(0)}%</span>
            </div>
          </div>
        </div>
      )}

      {/* 今日决策 */}
      <div className="sim-trade-section">
        <h4 className="sim-trade-section-title">今日决策</h4>
        <Table
          columns={decisionColumns}
          dataSource={simStatus?.decisions || []}
          rowKey="id"
          loading={loading}
          size="small"
          pagination={false}
          scroll={{ x: 700 }}
          locale={{ emptyText: '暂无决策' }}
        />
      </div>

      {/* 交易记录 */}
      <div className="sim-trade-section">
        <h4 className="sim-trade-section-title">交易记录</h4>
        <Table
          columns={recordColumns}
          dataSource={simStatus?.records || []}
          rowKey="id"
          loading={loading}
          size="small"
          pagination={{ pageSize: 10, showSizeChanger: false, showTotal: (total) => `共 ${total} 条` }}
          scroll={{ x: 690 }}
          locale={{ emptyText: '暂无交易记录' }}
        />
      </div>
    </div>
  );
}