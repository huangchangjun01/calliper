import { useState, useCallback } from 'react';
import PredictionCard from '@/components/PredictionCard';
import PredictionTable from '@/components/PredictionTable';
import AccuracyChart from '@/components/AccuracyChart';
import StockAccuracyChart from '@/components/StockAccuracyChart';
import FailureAnalysis from '@/components/FailureAnalysis';
import {
  usePredictionSummaries,
  usePredictionDetails,
  useAccuracyTrend,
  useStockAccuracyRanking,
  useFailureCases,
} from '@/services/predictions';
import type { PredictionPeriod } from '@/types';
import './Predictions.css';

export default function Predictions() {
  const [accuracyPeriod, setAccuracyPeriod] = useState<PredictionPeriod>('short');

  const { data: summaries = [] } = usePredictionSummaries();
  const { data: details = [], isLoading: detailsLoading } = usePredictionDetails();
  const { data: accuracyTrend, isLoading: accuracyLoading } = useAccuracyTrend(accuracyPeriod);
  const { data: stockAccuracy = [], isLoading: stockAccuracyLoading } = useStockAccuracyRanking();
  const { data: failureCases = [], isLoading: failuresLoading } = useFailureCases();

  const handleAccuracyPeriodChange = useCallback((period: PredictionPeriod) => {
    setAccuracyPeriod(period);
  }, []);

  return (
    <div className="predictions">
      {/* 顶部：预测概览面板 */}
      <section className="predictions-section">
        <h2 className="predictions-section-title">预测概览</h2>
        <div className="predictions-overview-grid">
          {summaries.map((summary) => (
            <PredictionCard
              key={summary.period}
              data={summary}
              period={summary.period}
            />
          ))}
        </div>
      </section>

      {/* 中部：预测列表 */}
      <section className="predictions-section">
        <PredictionTable data={details} loading={detailsLoading} />
      </section>

      {/* 底部：预测成功率统计 */}
      <section className="predictions-section">
        <h2 className="predictions-section-title">预测成功率统计</h2>

        <div className="predictions-accuracy-grid">
          <div className="predictions-accuracy-line">
            <AccuracyChart
              data={accuracyTrend}
              loading={accuracyLoading}
              onPeriodChange={handleAccuracyPeriodChange}
              currentPeriod={accuracyPeriod}
            />
          </div>
          <div className="predictions-accuracy-bar">
            <StockAccuracyChart
              data={stockAccuracy}
              loading={stockAccuracyLoading}
            />
          </div>
        </div>
      </section>

      {/* 底部：预测失败归因分析 */}
      <section className="predictions-section">
        <FailureAnalysis
          data={failureCases}
          loading={failuresLoading}
        />
      </section>
    </div>
  );
}