# Direction 2: Notification System（通知系统）

## 实现状态：❌ 待实现

[English](./notification.md)

## 目标描述

为 MiBeeHive 建立统一的通知机制，覆盖爬取完成、下载成功/失败、ISO 可用性检查、磁盘空间告警等关键事件。通过多渠道通知（Web UI、可选第三方集成），让管理员及时了解系统状态，无需主动轮询。

## 核心功能列表

- **事件触发通知**：爬取完成、下载完成/失败、ISO 新版本发现、磁盘空间不足
- **Web UI 通知中心**：浏览器内通知面板，支持已读/未读标记
- **通知历史**：持久化通知记录，支持按类型/时间检索
- **第三方通知集成**（可选）：企业微信、钉钉、Telegram Bot、邮件
- **通知规则配置**：用户可配置哪些事件触发通知、通知渠道
- **通知聚合**：同类通知聚合展示，避免通知风暴

## 技术方案概述

### 后端设计
- 新增 `internal/notifier/` 包，定义 `Notifier` 接口
- `Notification` 结构体：类型、级别、标题、内容、时间戳
- 内置 WebSocket 推送（实时通知），备选 SSE
- 通知存储复用现有 SQLite + 新增 `notifications` 表
- 集成点：CrawlManager 完成后回调、FileService 下载完成/失败回调、ISOService 版本检查、磁盘监控

### 前端设计
- `web/js/modules/notifications.js` — 通知面板组件
- 右上角铃铛图标 + 未读计数徽标
- 通知列表：分组展示，支持按类型筛选
- 增量更新：新通知到达时追加，不破坏列表 DOM

### 数据库设计
```sql
-- 新增 migration
CREATE TABLE notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,          -- 'crawl_complete', 'download_success', 'download_fail', 'iso_update', 'disk_warning'
    level TEXT NOT NULL,         -- 'info', 'warning', 'error'
    title TEXT NOT NULL,
    message TEXT,
    reference_id INTEGER,       -- 关联的业务 ID（project_id, file_id, iso_id 等）
    is_read INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## 优先级评估

| 功能 | 优先级 | 工作量 |
|------|--------|--------|
| Web UI 通知中心 | P0 | 2 天 |
| 事件触发集成 | P1 | 2 天 |
| 通知持久化与历史 | P1 | 1 天 |
| 第三方通知集成 | P2 | 3 天 |
| 通知规则配置 | P2 | 2 天 |

**总计预估工作量：10 人天**

## 验收标准
- 爬取完成自动触发通知，Web UI 实时显示
- 下载失败生成 error 级别通知
- 未读计数准确更新
- 通知历史可查询、可标记已读
- 通知聚合有效避免消息风暴
