# 为 MiBeeHive 做贡献

欢迎！我们很高兴您有兴趣为 MiBeeHive 项目做贡献。本文档提供了项目贡献指南。

> MiBeeHive 是**面向外部服务器的运维工具供应链平台**：它采集并持续更新运维工具，按标准协议对外供应给外部服务器。它**不是** 1Panel 那种本机应用商店——提功能时请牢记这一边界。

## 开发环境设置

### 前置要求
- 已安装 Go 1.26+（最新版）
- Git 用于版本控制

### 开始使用
```bash
git clone https://github.com/Mi-Bee-Studio/mibeehive.git
cd mibeehive
go mod download
```

### 构建和运行
```bash
# 构建主应用程序
go build -o mibeehive ./cmd/mibeehive

# 构建迁移工具
go build -o migrate ./cmd/migrate

# 运行应用程序
./mibeehive
```

## 代码风格

### Go 代码
- 遵循标准 Go 格式化和约定
- 使用适当的错误包装：`fmt.Errorf("context: %w", err)`
- 使用 `log/slog` 和键值对进行结构化日志记录
- 保持函数专注和单一用途
- 使用有意义的变量和函数名

### 错误处理
```go
// 正确
err := db.QueryRow("SELECT * FROM files WHERE id = ?", id).Scan(&file)
if err != nil {
    return nil, fmt.Errorf("数据库查询失败: %w", err)
}

// 错误
err := db.QueryRow("SELECT * FROM files WHERE id = ?", id).Scan(&file)
if err != nil {
    return nil, err
}
```

### 日志记录
```go
// 正确
log.Info("文件下载开始", "file_id", file.ID, "size", file.Size)
log.Debug("重试下载", "attempt", attempt, "max_attempts", maxAttempts)

// 错误
log.Println("开始下载文件", file.ID)
```

## 前端开发

### 技术栈
- **框架**: Preact + HTM（轻量级 React 替代品）
- **样式**: TailwindCSS 通过 CDN
- **图表**: Chart.js 通过 CDN
- **无 npm**: 所有前端代码都是使用 Preact bridge 的原生 JavaScript

### 代码指南
- 使用 CSS 变量（`--color-*`）而不是硬编码颜色
- 在 CSS 中绝不要使用 `!important`
- 使用 `data-id` 属性在定期更新期间进行 DOM 识别
- 优先使用目标 DOM 操作（textContent、classList、appendChild、remove）而不是 `innerHTML`
- 就地更新 Chart.js 实例：`chart.data = ...; chart.update('none')`
- 在数据刷新期间保留进度条 DOM 状态

### 目录结构
```
web/js/
├── core/         # 框架组件
├── layout/       # 共享 UI 组件
└── modules/      # 页面特定模块
```

## 数据库迁移

### 迁移规则
- **绝不修改** `migrations/001_init.sql`
- 始终创建**新的**迁移文件，使用连续编号
- 使用描述性名称：`002_add_user_table.sql`、`003_update_indexes.sql`
- 先在开发环境中测试迁移

### 迁移过程
```sql
-- 示例：添加新列
ALTER TABLE files ADD COLUMN download_url TEXT;

-- 示例：添加新表
CREATE TABLE IF NOT EXISTS downloads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id INTEGER,
    status TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (file_id) REFERENCES files(id)
);
```

## 测试

### 运行测试
```bash
# 运行所有测试
go test ./...

# 运行详细输出的测试
go test -v ./...

# 运行特定包测试
go test -v ./internal/crawler
go test -v ./internal/service

# 运行覆盖率测试
go test -cover ./...

# 运行静态分析
go vet ./...
```

### 测试指南
- 为新功能编写测试
- 对多个测试用例使用表驱动测试
- 适当模拟外部依赖
- 测试成功和失败场景

## 提交约定

### 提交消息格式
```
类型(范围): 简短描述

详细说明（如果需要）

# 修复 #123
# 关闭 #456
```

### 提交类型
- `feat`: 新功能
- `fix`: 错误修复
- `docs`: 文档更改
- `style`: 代码格式化
- `refactor`: 代码重构
- `test`: 测试相关更改
- `chore`: 构建或辅助工具更改

### 示例
```
feat(crawler): 添加 GitHub releases 源
fix(file-service): 正确处理网络超时
docs(readme): 更新安装说明
refactor(db): 优化查询性能
```

## 拉取请求流程

### 创建 PR 前
1. 确保所有测试通过：`go test ./...`
2. 运行静态分析：`go vet ./...`
3. 如需要更新文档
4. 检查任何需要处理的 TODO 注释
5. 确保您的更改遵循项目的代码风格

### PR 模板
```markdown
## 更改内容
- 对所做更改的简短描述
- 列出任何破坏性更改
- 包含任何相关的背景信息

## 测试
- 描述如何测试更改
- 包括添加的任何测试用例

## 检查清单
- [ ] 已添加/更新测试
- [ ] 已更新文档
- [ ] 代码遵循风格指南
- [ ] 无破坏性更改（除非有意为之）
```

### 审查流程
1. 提交您的 PR，附上清晰的描述
2. 确保通过 CI 检查
3. 及时处理任何审查评论
4. 保持 PR 聚焦于单个更改
5. 在审查中保持尊重和协作

## 开发工作流

### 分支策略
- `main`: 生产就绪的代码
- `develop`: 集成分支，用于 ongoing 开发
- 功能分支：从 `develop` 创建，合并回 `develop`

### 代码审查指南
- 建设性和尊重
- 专注于逻辑，而非个人偏好
- 提出改进建议，而不是只是批评
- 审查前测试您的更改
- 及时回应评论

## 报告问题

### 错误报告
使用 GitHub issue 模板并包含：
- 复现步骤
- 预期 vs 实际行为
- 环境详情（操作系统、Go 版本等）
- 相关日志或错误消息

### 功能请求
包括：
- 请求功能的清晰描述
- 用例和动机
- 任何实现建议
- 对现有功能的潜在影响

## 社区准则

- 包容和尊重
- 在可能时帮助新手
- 关注技术价值
- 遵循项目的行为准则
- 如果不确定，请提问

感谢您为 MiBeeHive 做贡献！🎉