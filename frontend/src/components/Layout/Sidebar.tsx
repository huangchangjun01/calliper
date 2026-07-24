import { memo } from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import type { SidebarMenuItem } from '@/types';

const menuItems: SidebarMenuItem[] = [
  { key: 'dashboard', label: '仪表盘', icon: '📊', path: '/' },
  { key: 'stocks', label: '股票检索', icon: '🔍', path: '/stocks' },
  { key: 'market', label: '实时行情', icon: '📈', path: '/market' },
  { key: 'trading', label: '交易面板', icon: '💹', path: '/trading' },
  { key: 'predictions', label: '预测分析', icon: '🤖', path: '/predictions' },
  { key: 'admin', label: '管理后台', icon: '⚙️', path: '/admin' },
];

interface SidebarProps {
  collapsed: boolean;
  onToggle: () => void;
}

const Sidebar = memo(function Sidebar({ collapsed, onToggle }: SidebarProps) {
  const location = useLocation();

  const isActive = (path: string) => {
    if (path === '/') {
      return location.pathname === '/';
    }
    return location.pathname.startsWith(path);
  };

  return (
    <aside
      className="layout-sidebar"
      style={{
        width: collapsed ? 'var(--sidebar-collapsed-width)' : 'var(--sidebar-width)',
        transition: 'width var(--transition-normal)',
        overflow: 'hidden',
      }}
    >
      {/* Logo 区域 */}
      <div className="sidebar-logo">
        {!collapsed && <span className="sidebar-logo-text">量化交易系统</span>}
        <button
          className="sidebar-toggle"
          onClick={onToggle}
          title={collapsed ? '展开菜单' : '收起菜单'}
        >
          {collapsed ? '▶' : '◀'}
        </button>
      </div>

      {/* 导航菜单 */}
      <nav className="sidebar-nav">
        {menuItems.map((item) => (
          <NavLink
            key={item.key}
            to={item.path}
            className={({ isActive: active }) =>
              `sidebar-menu-item ${active || isActive(item.path) ? 'active' : ''}`
            }
            title={collapsed ? item.label : undefined}
          >
            <span className="sidebar-menu-icon">{item.icon}</span>
            {!collapsed && <span className="sidebar-menu-label">{item.label}</span>}
          </NavLink>
        ))}
      </nav>

      {/* 底部版本号 */}
      <div className="sidebar-footer">
        {!collapsed ? (
          <span className="sidebar-version">v0.1.0</span>
        ) : (
          <span className="sidebar-version" title="v0.1.0">
            v0.1
          </span>
        )}
      </div>
    </aside>
  );
});

export default Sidebar;