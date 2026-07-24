import styles from './index.module.css';

interface MarketTabsProps {
  activeMarket: string;
  onChange: (market: string) => void;
}

const MARKET_TABS = [
  { key: '', label: '全部' },
  { key: 'SSE,SZSE,BSE', label: 'A股' },
  { key: 'HKEX', label: '港股' },
  { key: 'NYSE,NASDAQ,AMEX', label: '美股' },
  { key: 'TSE', label: '日股' },
  { key: 'LSE,Euronext,Xetra', label: '欧股' },
  { key: 'ASX,TSX,KRX', label: '其他' },
];

export default function MarketTabs({ activeMarket, onChange }: MarketTabsProps) {
  return (
    <div className={styles.tabs}>
      {MARKET_TABS.map((tab) => (
        <button
          key={tab.key}
          className={`${styles.tab} ${activeMarket === tab.key ? styles.active : ''}`}
          onClick={() => onChange(tab.key)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}