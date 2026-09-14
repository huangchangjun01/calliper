import { BrowserRouter, Routes, Route, Link } from 'react-router-dom';
import Layout from '@/components/Layout';
import AuthGuard from '@/components/AuthGuard';
import LoginPage from '@/pages/Login';
import RegisterPage from '@/pages/Register';
import Dashboard from '@/pages/Dashboard';
import StockSearch from '@/pages/StockSearch';
import StockDetail from '@/pages/StockDetail';
import Market from '@/pages/Market';
import TradingPanel from '@/pages/TradingPanel';
import Predictions from '@/pages/Predictions';
import AdminPanel from '@/pages/AdminPanel';
import Account from '@/pages/Account';

/** 404 兜底页（在 Layout 内渲染，保持侧边栏/顶栏） */
function NotFound() {
  return (
    <div className="not-found">
      <div className="not-found-code">404</div>
      <h1 className="not-found-title">页面不存在</h1>
      <p className="not-found-desc">您访问的地址不存在或已被移除。</p>
      <Link className="not-found-link" to="/">
        返回首页
      </Link>
    </div>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* 登录 / 注册页 —— 未认证用户入口 */}
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />

        {/* 受保护页面 —— 需要登录 */}
        <Route element={<AuthGuard />}>
          <Route element={<Layout />}>
            <Route path="/" element={<Dashboard />} />
            <Route path="/stocks" element={<StockSearch />} />
            <Route path="/stocks/:symbol" element={<StockDetail />} />
            <Route path="/market" element={<Market />} />
            <Route path="/trading" element={<TradingPanel />} />
            <Route path="/predictions" element={<Predictions />} />
            <Route path="/admin" element={<AuthGuard roles={['admin']} />}>
              <Route index element={<AdminPanel />} />
            </Route>
            <Route path="/account" element={<Account />} />
            {/* 兜底：未知路径显示 404 页 */}
            <Route path="*" element={<NotFound />} />
          </Route>
        </Route>
      </Routes>
    </BrowserRouter>
  );
}