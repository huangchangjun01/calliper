import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { useAuthStore } from '@/stores/authStore';
import type { User } from '@/types';

interface AuthGuardProps {
  /** 允许访问的角色列表；为空则仅校验登录态 */
  roles?: User['role'][];
}

/**
 * 认证守卫组件 —— 未登录用户重定向到登录页；若指定 roles 则校验角色权限，无权限重定向到主页。
 */
export default function AuthGuard({ roles }: AuthGuardProps) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const user = useAuthStore((s) => s.user);
  const location = useLocation();

  if (!isAuthenticated) {
    // 保存当前路径，登录成功后跳回
    return <Navigate to="/login" state={{ from: location.pathname }} replace />;
  }

  if (roles && roles.length > 0 && (!user || !roles.includes(user.role))) {
    // 无权限访问，重定向到主页
    return <Navigate to="/" replace />;
  }

  return <Outlet />;
}