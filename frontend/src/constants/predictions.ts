import type { PredictionPeriod } from '@/types';

/** 预测周期说明：主标签、预测未来交易日数、有效期天数与一句话描述 */
export interface PredictionPeriodInfo {
  label: string;
  /** 模型预测未来多少个交易日的走势 */
  horizonDays: number;
  /** 预测结果的判定有效期（天） */
  validityDays: number;
  /** 完整描述，用于悬停提示 */
  desc: string;
}

/**
 * 周期配置（与后端 periodDays 及 ML 训练参数保持一致）：
 * - short  短期   LSTM，预测未来 3 个交易日
 * - medium 中短期 XGBoost+LightGBM，预测未来 10 个交易日
 * - long   长期   Transformer，预测未来 30 个交易日
 */
export const PERIOD_INFO: Record<PredictionPeriod, PredictionPeriodInfo> = {
  short: {
    label: '短期',
    horizonDays: 3,
    validityDays: 3,
    desc: '预测未来 3 个交易日走势，有效期 3 天',
  },
  medium: {
    label: '中短期',
    horizonDays: 10,
    validityDays: 10,
    desc: '预测未来 10 个交易日走势，有效期 10 天',
  },
  long: {
    label: '长期',
    horizonDays: 30,
    validityDays: 30,
    desc: '预测未来 30 个交易日走势，有效期 30 天',
  },
};

/** 多周期合并提示文案，用于「周期」筛选栏等汇总区域的悬停说明 */
export const PERIOD_INFO_TIPS = Object.values(PERIOD_INFO)
  .map((p) => `${p.label}：${p.desc}`)
  .join('\n');