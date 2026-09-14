import type { AccountInfo } from '@/types';
import InfoTip from '@/components/common/InfoTip';

interface AccountOverviewProps {
  account: AccountInfo | null;
  loading?: boolean;
}

function formatAmount(value: number): string {
  if (value >= 1e8) return `${(value / 1e8).toFixed(2)}亿`;
  if (value >= 1e4) return `${(value / 1e4).toFixed(2)}万`;
  return value.toFixed(2);
}

function formatProfit(value: number): { text: string; className: string } {
  const sign = value > 0 ? '+' : '';
  const className = value > 0 ? 'profit-up' : value < 0 ? 'profit-down' : '';
  return { text: `${sign}${value.toFixed(2)}`, className };
}

export default function AccountOverview({ account, loading }: AccountOverviewProps) {
  if (!account && !loading) {
    return (
      <div className="account-overview">
        <div className="account-overview-empty">暂无账户数据</div>
      </div>
    );
  }

  if (loading || !account) {
    return (
      <div className="account-overview">
        <div className="account-overview-loading">加载中...</div>
      </div>
    );
  }

  const todayProfit = formatProfit(account.todayProfit);
  const totalProfit = formatProfit(account.totalProfit);

  const todayProfitPercent = account.todayProfitPercent;
  const todayProfitPercentSub = todayProfitPercent != null
    ? `${todayProfitPercent > 0 ? '+' : ''}${todayProfitPercent.toFixed(2)}%`
    : '--';

  const totalProfitPercent = account.totalProfitPercent;
  const totalProfitPercentSub = totalProfitPercent != null
    ? `${totalProfitPercent > 0 ? '+' : ''}${totalProfitPercent.toFixed(2)}%`
    : '--';

  const cards = [
    {
      label: '总资产',
      value: `¥ ${formatAmount(account.totalAsset)}`,
      className: '',
      tip: '现金 + 持仓市值',
    },
    {
      label: '可用资金',
      value: `¥ ${formatAmount(account.availableCash)}`,
      className: '',
      tip: '当前可自由用于下单的资金（持仓中资金不可用）',
    },
    {
      label: '持仓市值',
      value: `¥ ${formatAmount(account.marketValue)}`,
      className: '',
      tip: '当前持仓按最新价格计算的市值',
    },
    {
      label: '今日盈亏',
      value: `¥ ${todayProfit.text}`,
      className: todayProfit.className,
      sub: todayProfitPercentSub,
      tip: '当日产生的盈亏。今日收益率 = 今日盈亏 ÷ 昨日总资产 × 100%。',
    },
    {
      label: '总盈亏',
      value: `¥ ${totalProfit.text}`,
      className: totalProfit.className,
      sub: totalProfitPercentSub,
      tip: '账户累计盈亏（含已实现 + 未实现）。总收益率 = 总盈亏 ÷ 初始资金 × 100%。',
    },
    {
      label: '风险等级',
      value: account.riskLevel || '--',
      className: '',
      tip: '账户风险评估等级',
    },
  ];

  return (
    <div className="account-overview">
      <h3 className="account-overview-title">
        <InfoTip tip="模拟/真实交易账户的核心资产数据概览">账户资产</InfoTip>
      </h3>
      <div className="account-overview-cards">
        {cards.map((card) => (
          <div key={card.label} className="account-overview-card">
            <div className="account-overview-card-label">{card.label}</div>
            <InfoTip tip={card.tip}>
              <div className={`account-overview-card-value ${card.className}`}>
                {card.value}
              </div>
            </InfoTip>
            {card.sub && (
              <div className={`account-overview-card-sub ${card.className}`}>
                {card.sub}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}