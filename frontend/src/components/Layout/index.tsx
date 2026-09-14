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
  '/account': '账号设置',
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
  // 移动端抽屉显隐（与桌面 collapsed 独立，避免互相干扰）
  const [mobileOpen, setMobileOpen] = useState(false);

  const pageTitle = getPageTitle(location.pathname);

  useEffect(() => {
    const checkMobile = () => {
      const mobile = window.innerWidth < 768;
      setIsMobile(mobile);
      if (!mobile) {
        // 回到桌面端时复位移动抽屉
        setMobileOpen(false);
      }
    };
    checkMobile();
    window.addEventListener('resize', checkMobile);
    return () => window.removeEventListener('resize', checkMobile);
  }, []);

  // 路由切换时关闭移动端抽屉
  useEffect(() => {
    if (isMobile) {
      setMobileOpen(false);
    }
  }, [location.pathname, isMobile]);

  // 桌面端用 collapsed；移动端用 mobileOpen（默认收起，点汉堡展开）
  const sidebarCollapsed = isMobile ? !mobileOpen : collapsed;

  const handleToggle = useCallback(() => {
    if (isMobile) {
      setMobileOpen((open) => !open);
    } else {
      setCollapsed((prev) => !prev);
    }
  }, [isMobile]);

  // 移动端点击遮罩层关闭侧边栏
  const handleOverlayClick = useCallback(() => {
    setMobileOpen(false);
  }, []);

  return (
    <div className="layout">
      {/* 移动端遮罩：仅抽屉展开时显示 */}
      {isMobile && mobileOpen && (
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