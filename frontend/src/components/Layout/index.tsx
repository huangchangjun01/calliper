import { useState, useCallback, useEffect } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import Sidebar from './Sidebar';
import Header from './Header';

const pageTitles: Record<string, string> = {
  '/': '仪表盘',
  '/stocks': '股票检索',
  '/market': '实时行情',
  '/trading': '交易面板',
  '/predictions': '预测分析',
  '/admin': '管理后台',
};

function getPageTitle(pathname: string): string {
  // 精确匹配
  if (pageTitles[pathname]) return pageTitles[pathname];
  // 动态路由匹配 /stocks/:symbol
  if (pathname.startsWith('/stocks/')) return '股票详情';
  return '量化交易系统';
}

export default function Layout() {
  const [collapsed, setCollapsed] = useState(false);
  const location = useLocation();
  const [isMobile, setIsMobile] = useState(false);

  const pageTitle = getPageTitle(location.pathname);

  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener('resize', checkMobile);
    return () => window.removeEventListener('resize', checkMobile);
  }, []);

  // 移动端自动隐藏侧边栏
  const sidebarCollapsed = isMobile ? true : collapsed;

  const handleToggle = useCallback(() => {
    if (isMobile) {
      setIsMobile(false); // 移动端点击展开时临时切换
    } else {
      setCollapsed((prev) => !prev);
    }
  }, [isMobile]);

  // 移动端点击遮罩层关闭侧边栏
  const handleOverlayClick = useCallback(() => {
    setIsMobile(true);
  }, []);

  return (
    <div className="layout">
      {/* 移动端遮罩 */}
      {isMobile && !collapsed && (
        <div className="layout-overlay" onClick={handleOverlayClick} />
      )}

      {/* 侧边栏 */}
      <Sidebar collapsed={sidebarCollapsed} onToggle={handleToggle} />

      {/* 右侧主区域 */}
      <div className="layout-main">
        <Header
          collapsed={sidebarCollapsed}
          onToggle={handleToggle}
          title={pageTitle}
        />
        <main className="layout-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}