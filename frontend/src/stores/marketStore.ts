import { create } from 'zustand';
import type { StockQuote } from '@/types';

interface MarketState {
  stocks: Map<string, StockQuote>;
  updateStock: (symbol: string, quote: StockQuote) => void;
  updateStocks: (quotes: StockQuote[]) => void;
  removeStock: (symbol: string) => void;
  getStock: (symbol: string) => StockQuote | undefined;
  clearAll: () => void;
}

export const useMarketStore = create<MarketState>((set, get) => ({
  stocks: new Map(),

  updateStock: (symbol: string, quote: StockQuote) => {
    set((state) => {
      const next = new Map(state.stocks);
      next.set(symbol, quote);
      return { stocks: next };
    });
  },

  updateStocks: (quotes: StockQuote[]) => {
    set((state) => {
      const next = new Map(state.stocks);
      for (const quote of quotes) {
        next.set(quote.symbol, quote);
      }
      return { stocks: next };
    });
  },

  removeStock: (symbol: string) => {
    set((state) => {
      const next = new Map(state.stocks);
      next.delete(symbol);
      return { stocks: next };
    });
  },

  getStock: (symbol: string) => {
    return get().stocks.get(symbol);
  },

  clearAll: () => {
    set({ stocks: new Map() });
  },
}));