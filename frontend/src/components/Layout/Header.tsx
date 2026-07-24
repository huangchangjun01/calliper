import { memo, useState, useRef, useEffect } from 'react';
import { useThemeStore } from '@/stores/themeStore';
import { useAuthStore } from '@/stores/authStore';

interface HeaderProps {
  collapsed: boolean;
  onToggle: () => void;
  title: string;
}

const Header = memo(function Header({ collapsed, onToggle, title }: HeaderProps) {
  const { theme, toggleTheme } = useThemeStore();
  const { user, logout } = useAuthStore();
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setDropdownOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  return (
    <header className="layout-header">
      {/* 左侧：折叠按钮 */}
      <div className="header-left">
        <button
          className="header-collapse-btn"
          onClick={onToggle}
          title={collapsed ? '展开侧边栏' : '收起侧边栏'}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <line x1="3" y1="6" x2="21" y2="6" />
            <line x1="3" y1="12" x2="21" y2="12" />
            <line x1="3" y1="18" x2="21" y2="18" />
          </svg>
        </button>
      </div>

      {/* 中间：页面标题 */}
      <div className="header-center">
        <h2 className="header-title">{title}</h2>
      </div>

      {/* 右侧：主题切换 + 用户信息 */}
      <div className="header-right">
        {/* 主题切换 */}
        <button
          className="header-theme-btn"
          onClick={toggleTheme}
          title={theme === 'light' ? '切换暗色主题' : '切换亮色主题'}
        >
          {theme === 'light' ? (
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="5" />
              <line x1="12" y1="1" x2="12" y2="3" />
              <line x1="12" y1="21" x2="12" y2="23" />
              <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
              <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
              <line x1="1" y1="12" x2="3" y2="12" />
              <line x1="21" y1="12" x2="23" y2="12" />
              <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
              <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
            </svg>
          ) : (
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
            </svg>
          )}
        </button>

        {/* 用户下拉 */}
        <div className="header-user" ref={dropdownRef}>
          <button
            className="header-user-btn"
            onClick={() => setDropdownOpen(!dropdownOpen)}
          >
            <span className="header-user-avatar">
              {user?.username?.charAt(0).toUpperCase() || 'U'}
            </span>
            {user && <span className="header-user-name">{user.username}</span>}
            <svg
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              style={{ transform: dropdownOpen ? 'rotate(180deg)' : 'none', transition: 'transform 0.2s' }}
            >
              <polyline points="6 9 12 15 18 9" />
            </svg>
          </button>

          {dropdownOpen && (
            <div className="header-user-dropdown">
              <div className="dropdown-item user-info">
                <span className="user-info-name">{user?.username || '未登录'}</span>
                <span className="user-info-email">{user?.email || ''}</span>
              </div>
              <div className="dropdown-divider" />
              <button className="dropdown-item" onClick={() => { logout(); setDropdownOpen(false); }}>
                退出登录
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
});

export default Header;