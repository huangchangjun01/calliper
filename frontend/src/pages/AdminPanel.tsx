import { Tabs } from 'antd';
import {
  DatabaseOutlined,
  MonitorOutlined,
  UserOutlined,
  ExperimentOutlined,
} from '@ant-design/icons';
import DataSourceConfig from '@/components/DataSourceConfig';
import SystemMonitor from '@/components/SystemMonitor';
import UserManagement from '@/components/UserManagement';
import ModelManagement from '@/components/ModelManagement';
import './AdminPanel.css';

const tabItems = [
  {
    key: 'datasource',
    label: (
      <span>
        <DatabaseOutlined />
        数据源配置
      </span>
    ),
    children: <DataSourceConfig />,
  },
  {
    key: 'monitor',
    label: (
      <span>
        <MonitorOutlined />
        系统监控
      </span>
    ),
    children: <SystemMonitor />,
  },
  {
    key: 'users',
    label: (
      <span>
        <UserOutlined />
        用户管理
      </span>
    ),
    children: <UserManagement />,
  },
  {
    key: 'models',
    label: (
      <span>
        <ExperimentOutlined />
        模型管理
      </span>
    ),
    children: <ModelManagement />,
  },
];

export default function AdminPanel() {
  return (
    <div className="admin-panel">
      <Tabs
        defaultActiveKey="datasource"
        items={tabItems}
        size="large"
        className="admin-tabs"
      />
    </div>
  );
}