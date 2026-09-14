import type { ReactNode, CSSProperties } from 'react';
import { Spin, Empty, Button } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

interface DataStateProps {
  loading?: boolean;
  isEmpty?: boolean;
  emptyText?: string;
  /** 空态引导文案（显示在缺省文案下方） */
  emptyDescription?: string;
  /** 出错信息，非空时展示错误态 */
  error?: string | null;
  onRetry?: () => void;
  children?: ReactNode;
}

const wrapperStyle: CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  flexDirection: 'column',
  padding: '32px 16px',
  minHeight: 120,
  gap: 12,
};

/**
 * 统一的三态数据容器：loading（Spin）/ 内容 / empty（引导文案）/ error（错误信息 + 重试）。
 * 用于避免任何业务卡片或列表块出现「空白」状态。
 */
export default function DataState({
  loading,
  isEmpty,
  emptyText = '暂无数据',
  emptyDescription,
  error,
  onRetry,
  children,
}: DataStateProps) {
  if (loading) {
    return (
      <div style={wrapperStyle} className="data-state data-state-loading">
        <Spin />
      </div>
    );
  }

  if (error) {
    return (
      <div style={wrapperStyle} className="data-state data-state-error">
        <span style={{ color: 'var(--color-error)', fontSize: 13 }}>{error}</span>
        {onRetry && (
          <Button size="small" icon={<ReloadOutlined />} onClick={onRetry}>
            重试
          </Button>
        )}
      </div>
    );
  }

  if (isEmpty) {
    return (
      <div style={wrapperStyle} className="data-state data-state-empty">
        <Empty
          description={emptyText}
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        >
          {emptyDescription && (
            <span style={{ color: 'var(--text-tertiary)', fontSize: 12 }}>
              {emptyDescription}
            </span>
          )}
        </Empty>
      </div>
    );
  }

  return <>{children}</>;
}