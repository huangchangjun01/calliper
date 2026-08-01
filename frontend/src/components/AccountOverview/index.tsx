import type { AccountInfo } from '@/types';

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
    },
    {
      label: '可用资金',
      value: `¥ ${formatAmount(account.availableCash)}`,
      className: '',
    },
    {
      label: '持仓市值',
      value: `¥ ${formatAmount(account.marketValue)}`,
      className: '',
    },
    {
      label: '今日盈亏',
      value: `¥ ${todayProfit.text}`,
      className: todayProfit.className,
      sub: todayProfitPercentSub,
    },
    {
      label: '总盈亏',
      value: `¥ ${totalProfit.text}`,
      className: totalProfit.className,
      sub: totalProfitPercentSub,
    },
    {
      label: '风险等级',
      value: account.riskLevel || '--',
      className: '',
    },
  ];

  return (
    <div className="account-overview">
      <h3 className="account-overview-title">账户资产</h3>
      <div className="account-overview-cards">
        {cards.map((card) => (
          <div key={card.label} className="account-overview-card">
            <div className="account-overview-card-label">{card.label}</div>
            <div className={`account-overview-card-value ${card.className}`}>
              {card.value}
            </div>
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