# Direction 2: Notification System (通知系统)

## Status: ❌ Not Started

[中文](./notification_zh.md)

## Goal

Establish a unified notification mechanism for MiBeeHive, covering key events such as crawl completion, download success/failure, ISO availability checks, and disk space warnings. Through multi-channel notifications (Web UI, optional third-party integration), enable administrators to stay informed of system status without active polling.

## Core Features

- **Event-triggered notifications**: Crawl completion, download completion/failure, new ISO version discovery, insufficient disk space
- **Web UI notification center**: In-browser notification panel, supports read/unread marking
- **Notification history**: Persistent notification records, support retrieval by type/time
- **Third-party notification integration** (optional): WeChat Work, DingTalk, Telegram Bot, Email
- **Notification rule configuration**: Users can configure which events trigger notifications, notification channels
- **Notification aggregation**: Similar notifications aggregated display, avoid notification storms

## Technical Solution

### Backend Design
- New `internal/notifier/` package, define `Notifier` interface
- `Notification` structure: type, level, title, content, timestamp
- Built-in WebSocket push (real-time notifications), alternative SSE
- Notification storage reuses existing SQLite + new `notifications` table
- Integration points: CrawlManager completion callback, FileService download completion/failure callback, ISOService version check, disk monitoring

### Frontend Design
- `web/js/modules/notifications.js` — Notification panel component
- Bell icon + unread count badge in top-right corner
- Notification list: grouped display, support filtering by type
- Incremental updates: append new notifications, don't destroy list DOM

### Database Design
```sql
-- New migration
CREATE TABLE notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,          -- 'crawl_complete', 'download_success', 'download_fail', 'iso_update', 'disk_warning'
    level TEXT NOT NULL,         -- 'info', 'warning', 'error'
    title TEXT NOT NULL,
    message TEXT,
    reference_id INTEGER,       -- Related business ID (project_id, file_id, iso_id, etc.)
    is_read INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## Priority Assessment

| Feature | Priority | Workload |
|---------|----------|----------|
| Web UI notification center | P0 | 2 days |
| Event trigger integration | P1 | 2 days |
| Notification persistence and history | P1 | 1 day |
| Third-party notification integration | P2 | 3 days |
| Notification rule configuration | P2 | 2 days |

**Total estimated workload: 10 person-days**

## Acceptance Criteria
- Crawl completion automatically triggers notifications, displayed in real-time in Web UI
- Download failures generate error-level notifications
- Unread count updates accurately
- Notification history is queryable and markable as read
- Notification aggregation effectively avoids message storms
