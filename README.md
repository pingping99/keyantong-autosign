# AbleSci 自动签到 (keyantong-autosign)

科研通 (AbleSci.com) 自动签到工具，支持多账户管理、随机签到窗口、Docker 部署。

## 功能特性

- 🔄 **多账户支持** — 同时管理多个账户，独立状态跟踪
- 🎲 **随机签到窗口** — 每日生成随机签到时间，避免检测
- ⏰ **工作时间限制** — 仅在设定时间范围内执行签到
- 🔁 **自动重试** — 登录过期自动重登、失败自动重试
- 📝 **日志轮转** — 自动管理日志文件大小
- 🐳 **Docker 支持** — 开箱即用的容器化部署

## 快速开始

### 方式一：config.yml（推荐）

```bash
cp config.yaml.example config.yml
# 编辑 config.yml，填入账户信息
go run main.go
```

### 方式二：环境变量

```bash
export ABLESCI_EMAIL=your@example.com
export ABLESCI_PASSWORD=your_password
go run main.go
```

### 方式三：Docker

```bash
cp .env.example .env
# 编辑 .env，填入账户信息
docker compose up -d --build
```

## 配置

### 配置优先级

```
环境变量  >  config.yml  >  默认值
```

### 配置项

| 配置项 | 环境变量 | YAML 键名 | 默认值 | 说明 |
|--------|----------|-----------|--------|------|
| 账户邮箱 | `ABLESCI_EMAIL` | `accounts[].email` | — | 必填 |
| 账户密码 | `ABLESCI_PASSWORD` | `accounts[].password` | — | 必填 |
| 数据目录 | `DATA_DIR` | `data_dir` | `./data` | 日志和状态文件 |
| 检查间隔 | `CHECK_INTERVAL` | `check_interval` | `30m` | Go duration 格式 |
| 重试间隔 | `RETRY_INTERVAL` | `retry_interval` | `10m` | 签到失败后重试 |
| 强制启动签到 | `FORCE_SIGN_ON_START` | `force_sign_on_start` | `true` | 启动时立即签到 |
| 时区 | `TZ` | `timezone` | `Asia/Shanghai` | — |
| 工作时间起始 | `EARLY_HOUR_THRESHOLD` | `early_hour_threshold` | `8` | 小时 (0-23) |
| 工作时间结束 | `LATE_HOUR_THRESHOLD` | `late_hour_threshold` | `22` | 小时 (0-23) |

### 账户配置优先级

```
环境变量 (ABLESCI_EMAIL/PASSWORD)  >  config.yml accounts  >  data/accounts.json
```

**config.yml 方式（多账户）：**

```yaml
accounts:
  - email: user1@example.com
    password: password1
  - email: user2@example.com
    password: password2
```

**accounts.json 方式：**

```json
[
  {"email": "user1@example.com", "password": "password1"},
  {"email": "user2@example.com", "password": "password2"}
]
```

## 项目结构

```
keyantong/
├── main.go              # 入口：组装依赖、启动定时检查
├── config/
│   ├── config.go        # 配置加载（优先级解析）、账户加载
│   └── yaml.go          # 轻量级 YAML 解析器
├── core/
│   ├── model.go         # 数据模型（SignState, SignRecord）
│   ├── store.go         # 状态持久化（文件存储）
│   ├── service.go       # HTTP 客户端 + AbleSci API
│   ├── signer.go        # 签到编排（重试、窗口判断）
│   └── timeutil.go      # 时间工具、随机窗口生成
├── config.yaml.example   # 配置模板
├── docker-compose.yml   # Docker 部署
└── Dockerfile
```

## 签到流程

1. **启动** — 加载配置、初始化各账户签到器
2. **强制签到**（可选）— 启动时立即签到（尊重工作时间）
3. **定时检查** — 每 `CHECK_INTERVAL` 检查一次
4. **随机窗口** — 每日生成随机签到时间，持久化到状态文件
5. **执行签到** — 到达随机时间后，添加抖动延迟，执行签到
6. **自动重登** — 会话过期时自动重新登录并重试
7. **状态记录** — 保存签到结果，防止重复签到

## 开发

```bash
# 构建
go build -o ablesci-sign.exe

# 运行
go run main.go

# Docker
docker compose up -d --build
docker compose logs -f ablesci-sign
```

## 依赖

- Go 1.21+
- 零外部依赖（仅使用标准库）

## 许可

MIT License
