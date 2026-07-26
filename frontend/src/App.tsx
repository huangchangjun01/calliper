import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from '@/components/Layout';
import AuthGuard from '@/components/AuthGuard';
import LoginPage from '@/pages/Login';
import Dashboard from '@/pages/Dashboard';
import StockSearch from '@/pages/StockSearch';
import StockDetail from '@/pages/StockDetail';
import Market from '@/pages/Market';
import TradingPanel from '@/pages/TradingPanel';
import Predictions from '@/pages/Predictions';
import AdminPanel from '@/pages/AdminPanel';

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* 登录页 —— 未认证用户入口 */}
        <Route path="/login" element={<LoginPage />} />

        {/* 受保护页面 —— 需要登录 */}
        <Route element={<AuthGuard />}>
          <Route element={<Layout />}>
            <Route path="/" element={<Dashboard />} />
            <Route path="/stocks" element={<StockSearch />} />
            <Route path="/stocks/:symbol" element={<StockDetail />} />
            <Route path="/market" element={<Market />} />
            <Route path="/trading" element={<TradingPanel />} />
            <Route path="/predictions" element={<Predictions />} />
            <Route path="/admin" element={<AdminPanel />} />
          </Route>
        </Route>
      </Routes>
    </BrowserRouter>
  );
}