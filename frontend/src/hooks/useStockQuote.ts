import { useState, useEffect, useRef, useCallback } from 'react';
import type { StockQuote, WsMessage } from '@/types';
import useWebSocket from '@/hooks/useWebSocket';
import useWsConnectionStatus from '@/hooks/useWsConnectionStatus';
import api from '@/services/api';

interface UseStockQuoteOptions {
  /** 暂停实时刷新：暂停期间忽略行情推送，不产生重新渲染 */
  paused?: boolean;
}

interface UseStockQuoteSnapshot {
  stocks: Map<string, StockQuote>;
  previousStocks: Map<string, StockQuote>;
  changedSymbols: Set<string>;
}

interface UseStockQuoteReturn extends UseStockQuoteSnapshot {}

// 将后端 MarketData 格式映射为前端 StockQuote 格式
function mapMarketData(data: Record<string, unknown>): StockQuote {
  const ts = data.timestamp as string;
  return {
    symbol: (data.symbol as string) ?? '',
    name: (data.name as string) ?? '',
    price: (data.price as number) ?? 0,
    open: (data.open as number) ?? 0,
    high: (data.high as number) ?? 0,
    low: (data.low as number) ?? 0,
    preClose: (data.pre_close as number) ?? 0,
    volume: (data.volume as number) ?? 0,
    amount: (data.amount as number) ?? 0,
    change: (data.change as number) ?? 0,
    changePercent: (data.change_percent as number) ?? 0,
    timestamp: ts ? new Date(ts).getTime() : Date.now(),
  };
}

export default function useStockQuote(
  symbols: string[],
  options: UseStockQuoteOptions = {}
): UseStockQuoteReturn {
  const { paused = false } = options;
  const stocksRef = useRef<Map<string, StockQuote>>(new Map());
  const previousStocksRef = useRef<Map<string, StockQuote>>(new Map());
  const changedSymbolsRef = useRef<Set<string>>(new Set());

  // 稳定快照：仅在批量 flush 后更新一次引用，避免每次渲染生成新 Map/Set 破坏下游 useMemo/effect
  const [snapshot, setSnapshot] = useState<UseStockQuoteSnapshot>(() => ({
    stocks: new Map(),
    previousStocks: new Map(),
    changedSymbols: new Set(),
  }));
  const flushRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const scheduleFlush = useCallback(() => {
    if (flushRef.current) return;
    flushRef.current = setTimeout(() => {
      flushRef.current = null;
      setSnapshot({
        stocks: new Map(stocksRef.current),
        previousStocks: new Map(previousStocksRef.current),
        changedSymbols: new Set(changedSymbolsRef.current),
      });
    }, 0);
  }, []);

  const channels = symbols.map((s) => `stock:${s}`);

  const handleMessage = useCallback((message: WsMessage) => {
    if (message.type !== 'quote' || !message.data) return;

    const quote = mapMarketData(message.data as Record<string, unknown>);
    if (!quote?.symbol) return;

    applyQuote(quote);
  }, []);

  // 将单条行情写入本地 ref / Map（纯内存副作用，不直接触发渲染）
  const applyQuote = useCallback((quote: StockQuote) => {
    const prev = stocksRef.current.get(quote.symbol);
    if (prev) {
      previousStocksRef.current.set(quote.symbol, prev);
    }

    stocksRef.current.set(quote.symbol, quote);

    changedSymbolsRef.current = new Set(changedSymbolsRef.current);
    changedSymbolsRef.current.add(quote.symbol);

    clearChangedTimer(quote.symbol);
    scheduleFlush();
  }, [scheduleFlush]);

  const clearChangedTimersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const clearChangedTimer = useCallback((symbol: string) => {
    const prev = clearChangedTimersRef.current.get(symbol);
    if (prev) clearTimeout(prev);
    const timer = setTimeout(() => {
      changedSymbolsRef.current = new Set(
        [...changedSymbolsRef.current].filter((s) => s !== symbol)
      );
      clearChangedTimersRef.current.delete(symbol);
      scheduleFlush();
    }, 500);
    clearChangedTimersRef.current.set(symbol, timer);
  }, [scheduleFlush]);

  useWebSocket(channels, {
    autoConnect: true,
    paused,
    onMessage: handleMessage,
  });

  // REST API 回退：定时拉取行情数据（分批，每批最多 50 只）。
  // 以 ref 挂载，供重连/恢复时增量同步复用，避免全页闪烁。
  const symbolsRef = useRef(symbols);
  symbolsRef.current = symbols;
  const pausedRef = useRef(paused);
  pausedRef.current = paused;
  const fetchQuotes = useCallback(async () => {
    if (pausedRef.current) return; // 暂停期间不主动拉取
    const syms = symbolsRef.current;
    if (syms.length === 0) return;
    const BATCH_SIZE = 50;
    try {
      for (let i = 0; i < syms.length; i += BATCH_SIZE) {
        const batch = syms.slice(i, i + BATCH_SIZE);
        const data = await api.post<{ count: number; data: Record<string, unknown>[] }>(
          '/market/realtime/batch',
          { symbols: batch }
        );
        if (data?.data) {
          for (const item of data.data) {
            applyQuote(mapMarketData(item));
          }
        }
      }
    } catch {
      // 静默失败
    }
  }, [applyQuote]);

  // 首次挂载 + 定时（每 10 秒）拉取一次；暂停时不拉取但定时器仍在以保持简单
  useEffect(() => {
    if (symbols.length === 0) return;
    const interval = setInterval(() => {
      if (!pausedRef.current) fetchQuotes();
    }, 10000);
    return () => clearInterval(interval);
  }, [symbols.join(','), fetchQuotes]);

  // 暂停 -> 恢复：恢复时做一次全量同步（增量刷新，避免整页闪烁）
  const prevPausedRef = useRef(paused);
  if (prevPausedRef.current !== paused) {
    prevPausedRef.current = paused;
    if (!paused) fetchQuotes();
  }

  // 重连成功：增量拉取一次最新行情（增量刷新，避免全页闪烁）
  const { isConnected } = useWsConnectionStatus();
  const hadOpenRef = useRef(false);
  useEffect(() => {
    if (isConnected) {
      if (hadOpenRef.current) {
        fetchQuotes();
      }
      hadOpenRef.current = true;
    }
  }, [isConnected, fetchQuotes]);

  useEffect(() => {
    return () => {
      stocksRef.current.clear();
      previousStocksRef.current.clear();
      changedSymbolsRef.current.clear();
      clearChangedTimersRef.current.forEach((t) => clearTimeout(t));
      clearChangedTimersRef.current.clear();
      if (flushRef.current) {
        clearTimeout(flushRef.current);
        flushRef.current = null;
      }
    };
  }, []);

  return {
    stocks: snapshot.stocks,
    previousStocks: snapshot.previousStocks,
    changedSymbols: snapshot.changedSymbols,
  };
}