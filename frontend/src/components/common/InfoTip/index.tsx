import { Tooltip } from 'antd';
import { QuestionCircleOutlined } from '@ant-design/icons';
import type { ReactNode } from 'react';
import './index.css';

interface InfoTipProps {
  /** 提示说明文字：该区域块的含义与作用（支持换行 \n） */
  tip: string;
  /** 被包裹的数值区域块：鼠标移入时显示说明 */
  children?: ReactNode;
  /** 是否额外显示问号图标以增强可发现性（默认 true） */
  showIcon?: boolean;
  /** 弹出层标题（可选，如 "最近同步"） */
  title?: string;
  /** 弹出方向 */
  placement?: 'top' | 'bottom' | 'left' | 'right';
  className?: string;
}

/**
 * InfoTip —— 新手引导提示组件
 * 用法 1：包裹数值区域块，鼠标移入区块即显示其含义
 *   <InfoTip tip="近 7 日预测准确率 = 近 7 天已到期预测中判定正确的比例">
 *     <Statistic title="近7日准确率" value={50} suffix="%" />
 *   </InfoTip>
 * 用法 2：单独作为问号图标，附着在标题/文案旁
 *   <h3>准确率 <InfoTip tip="..." /></h3>
 */
export default function InfoTip({
  tip,
  children,
  showIcon = false,
  title,
  placement = 'top',
  className,
}: InfoTipProps) {
  const content = (
    <div className="infotip">
      {title && <div className="infotip-title">{title}</div>}
      <div className="infotip-body">{tip}</div>
    </div>
  );

  if (!children) {
    // 无可包裹区域时尊重 showIcon：默认 false 不渲染任何内容，避免页面残留问号
    if (!showIcon) return null;
    return (
      <Tooltip title={content} placement={placement} overlayClassName="infotip-overlay" mouseEnterDelay={0.15}>
        <QuestionCircleOutlined className={`infotip-icon ${className ?? ''}`} />
      </Tooltip>
    );
  }

  return (
    <Tooltip title={content} placement={placement} overlayClassName="infotip-overlay" mouseEnterDelay={0.15}>
      <span className={`infotip-wrap ${className ?? ''}`}>
        {children}
        {showIcon && <QuestionCircleOutlined className="infotip-icon" />}
      </span>
    </Tooltip>
  );
}
