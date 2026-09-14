import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { register } from '@/services/account';

const styles: Record<string, React.CSSProperties> = {
  page: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: '100vh',
    background: 'linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%)',
    position: 'relative',
    zIndex: 1,
  },
  card: {
    width: 400,
    maxWidth: '90vw',
    padding: '48px 40px',
    background: '#ffffff',
    borderRadius: 12,
    boxShadow: '0 8px 40px rgba(0,0,0,0.2)',
    position: 'relative',
    zIndex: 2,
  },
  header: {
    textAlign: 'center' as const,
    marginBottom: 36,
  },
  title: {
    fontSize: 28,
    fontWeight: 700,
    color: 'rgba(0,0,0,0.88)',
    margin: '0 0 8px',
  },
  subtitle: {
    fontSize: 14,
    color: 'rgba(0,0,0,0.45)',
    margin: 0,
  },
  error: {
    padding: '10px 14px',
    background: '#fff2f0',
    border: '1px solid #ffccc7',
    borderRadius: 6,
    color: '#ff4d4f',
    fontSize: 13,
    marginBottom: 16,
  },
  field: {
    marginBottom: 20,
  },
  label: {
    display: 'block',
    fontSize: 14,
    fontWeight: 500,
    color: 'rgba(0,0,0,0.65)',
    marginBottom: 6,
  },
  input: {
    width: '100%',
    height: 42,
    padding: '0 14px',
    border: '1px solid #d9d9d9',
    borderRadius: 6,
    fontSize: 14,
    color: 'rgba(0,0,0,0.88)',
    background: '#ffffff',
    outline: 'none',
    boxSizing: 'border-box' as const,
    transition: 'border-color 0.2s, box-shadow 0.2s',
  },
  inputFocus: {
    borderColor: '#1677ff',
    boxShadow: '0 0 0 2px rgba(22, 119, 255, 0.15)',
  },
  btn: {
    width: '100%',
    height: 44,
    marginTop: 4,
    background: '#1677ff',
    color: '#ffffff',
    border: 'none',
    borderRadius: 6,
    fontSize: 16,
    fontWeight: 500,
    cursor: 'pointer',
    transition: 'background 0.2s',
  },
  btnDisabled: {
    width: '100%',
    height: 44,
    marginTop: 4,
    background: '#1677ff',
    color: '#ffffff',
    border: 'none',
    borderRadius: 6,
    fontSize: 16,
    fontWeight: 500,
    opacity: 0.6,
    cursor: 'not-allowed',
  },
  footer: {
    marginTop: 20,
    textAlign: 'center' as const,
    fontSize: 14,
    color: 'rgba(0,0,0,0.45)',
  },
  link: {
    color: '#1677ff',
    cursor: 'pointer',
    textDecoration: 'none',
    marginLeft: 4,
  },
};

export default function RegisterPage() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [focusedField, setFocusedField] = useState<string | null>(null);
  const navigate = useNavigate();

  const getInputStyle = (fieldName: string): React.CSSProperties => {
    if (focusedField === fieldName) {
      return { ...styles.input, ...styles.inputFocus };
    }
    return styles.input;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!username.trim() || !email.trim() || !password || !confirmPassword) {
      setError('请填写所有必填项');
      return;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
      setError('请输入有效的邮箱地址');
      return;
    }
    if (password.length < 6) {
      setError('密码长度不能少于6位');
      return;
    }
    if (password !== confirmPassword) {
      setError('两次输入的密码不一致');
      return;
    }

    setLoading(true);
    try {
      await register({
        username: username.trim(),
        email: email.trim(),
        password,
      });
      navigate('/login', { state: { registered: true } });
    } catch (err) {
      setError(err instanceof Error ? err.message : '注册失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={styles.page}>
      <div style={styles.card}>
        <div style={styles.header}>
          <h1 style={styles.title}>创建账号</h1>
          <p style={styles.subtitle}>Quantitative Trading System</p>
        </div>

        <form onSubmit={handleSubmit}>
          {error && <div style={styles.error}>{error}</div>}

          <div style={styles.field}>
            <label style={styles.label} htmlFor="username">
              用户名
            </label>
            <input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              onFocus={() => setFocusedField('username')}
              onBlur={() => setFocusedField(null)}
              placeholder="请输入用户名"
              autoComplete="username"
              autoFocus
              disabled={loading}
              style={getInputStyle('username')}
            />
          </div>

          <div style={styles.field}>
            <label style={styles.label} htmlFor="email">
              邮箱
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              onFocus={() => setFocusedField('email')}
              onBlur={() => setFocusedField(null)}
              placeholder="请输入邮箱"
              autoComplete="email"
              disabled={loading}
              style={getInputStyle('email')}
            />
          </div>

          <div style={styles.field}>
            <label style={styles.label} htmlFor="password">
              密码
            </label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onFocus={() => setFocusedField('password')}
              onBlur={() => setFocusedField(null)}
              placeholder="请输入密码（至少6位）"
              autoComplete="new-password"
              disabled={loading}
              style={getInputStyle('password')}
            />
          </div>

          <div style={styles.field}>
            <label style={styles.label} htmlFor="confirmPassword">
              确认密码
            </label>
            <input
              id="confirmPassword"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              onFocus={() => setFocusedField('confirmPassword')}
              onBlur={() => setFocusedField(null)}
              placeholder="请再次输入密码"
              autoComplete="new-password"
              disabled={loading}
              style={getInputStyle('confirmPassword')}
            />
          </div>

          <button
            type="submit"
            style={loading ? styles.btnDisabled : styles.btn}
            disabled={loading}
          >
            {loading ? '注册中...' : '注 册'}
          </button>
        </form>

        <div style={styles.footer}>
          已有账号？
          <span
            style={styles.link}
            onClick={() => navigate('/login')}
          >
            去登录
          </span>
        </div>
      </div>
    </div>
  );
}
