import { Card, Table, Tag, Progress } from 'antd';
import {
  CheckCircleFilled,
  CloseCircleFilled,
  ThunderboltOutlined,
  DatabaseOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import InfoTip from '@/components/common/InfoTip';
import {
  useServiceHealth,
  useErrorLogs,
  useDataLatency,
  type ServiceHealth,
  type ErrorLog,
} from '@/services/admin';
import dayjs from 'dayjs';

// 格式化服务心跳时间：空/无效/Go 零时间（年份<2000）显示 '-'
function formatHeartbeat(value?: string | null): string {
  if (!value) return '-';
  const d = dayjs(value);
  if (!d.isValid() || d.year() < 2000) return '-';
  return d.format('HH:mm:ss');
}

export default function SystemMonitor() {
  const { data: services = [] } = useServiceHealth();
  const { data: errorLogs = [] } = useErrorLogs();
  const { data: latency } = useDataLatency();

  const serviceIcon: Record<ServiceHealth['service'], React.ReactNode> = {
    gateway: <ThunderboltOutlined />,
    market: <DatabaseOutlined />,
    prediction: <ThunderboltOutlined />,
    engine: <ThunderboltOutlined />,
  };

  const errorColumns: ColumnsType<ErrorLog> = [
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
      render: (ts: string) => dayjs(ts).format('HH:mm:ss'),
    },
    {
      title: '服务',
      dataIndex: 'service',
      key: 'service',
      width: 120,
      render: (s: string) => <Tag>{s}</Tag>,
    },
    {
      title: '错误信息',
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
    },
  ];

  return (
    <div className="admin-section">
      <div className="admin-section-header">
        <h3>系统监控</h3>
      </div>

      {/* 服务健康卡片 */}
      <div className="admin-health-grid">
        {services.map((svc) => (
          <Card key={svc.service} size="small" className="admin-health-card">
            <div className="admin-health-card-header">
              <span className="admin-health-icon">{serviceIcon[svc.service]}</span>
              <span className="admin-health-name">{svc.name}</span>
            </div>
            <div className="admin-health-body">
              <div className="admin-health-status">
                {svc.status === 'running' ? (
                  <>
                    <CheckCircleFilled style={{ color: 'var(--color-success)' }} />
                    <span className="admin-health-running">运行中</span>
                  </>
                ) : (
                  <>
                    <CloseCircleFilled style={{ color: 'var(--color-error)' }} />
                    <span className="admin-health-stopped">已停止</span>
                  </>
                )}
              </div>
              <div className="admin-health-metrics">
                <div className="admin-health-metric">
                  <span className="admin-health-label">延迟</span>
                  <span className="admin-health-value">
                    {svc.status === 'running' ? `${svc.latency}ms` : '-'}
                  </span>
                </div>
                <div className="admin-health-metric">
                  <span className="admin-health-label">最后心跳</span>
                  <span className="admin-health-value">
                    {formatHeartbeat(svc.lastHeartbeat)}
                  </span>
                </div>
              </div>
            </div>
          </Card>
        ))}
      </div>

      {/* 数据延迟监控 */}
      {latency && (
        <div className="admin-latency-row">
          <Card size="small" className="admin-latency-card">
            <div className="admin-latency-label">
              <InfoTip tip="缓存命中率；命中率越高表示更多数据直接走缓存、读取更快">Redis 命中率</InfoTip>
            </div>
            {Number.isFinite(latency.redisHitRate) ? (
              <Progress
                percent={Math.round(latency.redisHitRate * 100)}
                size="small"
                strokeColor={
                  latency.redisHitRate > 0.9 ? 'var(--color-success)' : 'var(--color-warning)'
                }
              />
            ) : (
              <span className="admin-latency-empty">暂无数据</span>
            )}
          </Card>
        </div>
      )}

      {/* 错误日志 */}
      <div className="admin-section-header">
        <h3>最近错误日志</h3>
      </div>
      <Table
        columns={errorColumns}
        dataSource={errorLogs}
        rowKey="id"
        pagination={false}
        size="small"
        locale={{ emptyText: '暂无数据' }}
      />
    </div>
  );
}