# AbleSci 自动签到脚本

基于 Go 语言开发的科研通（AbleSci.com）自动签到脚本。项目已在 GitHub 开源：<https://github.com/pingping99/keyantong-autosign>。

## 功能特性

- ✅ 自动登录
- ✅ 自动签到
- ✅ 支持 CSRF 令牌验证
- ✅ Cookie 会话管理
- ✅ 详细的签到结果显示
- ✅ 首次使用自动登录签到
- ✅ 动态签到时间（避免固定时间被检测）
- ✅ 登录状态检查与自动重登录

## 项目结构

```
keyantong/
├── main.go              # 主程序入口
├── go.mod               # Go 模块依赖
├── config/
│   └── config.go        # 配置加载
├── client/
│   └── client.go        # HTTP 客户端封装
├── service/
│   └── sign.go          # 登录和签到业务逻辑
├── signer/
│   └── signer.go        # 签到调度逻辑
├── scheduler/
│   └── window.go        # 动态窗口生成
└── interface.md         # 接口文档
```

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 配置账号

使用环境变量设置账号信息：

```bash
export ABLESCI_EMAIL="your_email@example.com"
export ABLESCI_PASSWORD="your_password"
```

### 3. 运行脚本

```bash
go run main.go
```

或编译后运行：

```bash
go build -o ablesci-sign.exe
./ablesci-sign.exe
```

## 运行示例

```
=== AbleSci 自动签到脚本 ===
正在登录...
✓ 登录成功
正在签到...
✓ 签到成功，您已连续签到 1 次，本次获得 20 积分。
  连续签到: 1 次
  本次获得: 20 积分

签到完成!
```

## 注意事项

- 请妥善保管账号信息，不要将环境变量或配置文件提交到版本控制系统
- 建议配合定时任务（如 cron、Windows 任务计划程序）实现每日自动签到
- 本脚本仅供学习交流使用
- 首次运行时会自动执行登录与签到
- 签到状态保存在 `data/state.json`
- 程序运行时会自动检查登录状态，如果会话失效会自动重新登录
- 程序采用动态签到时间，每天在配置的时间窗口内随机选择一个时间签到，避免固定时间被检测

## 定时任务配置

### Windows 任务计划程序

1. 打开"任务计划程序"
2. 创建基本任务，设置每天运行
3. 操作选择"启动程序"，选择编译后的 exe 文件
4. 完成配置

### Linux/Mac Cron

```bash
# 每天早上 8:00 执行签到
0 8 * * * cd /path/to/keyantong && ./ablesci-sign >> sign.log 2>&1
```

## Docker Compose 安装

1. 克隆源码：

```bash
git clone https://github.com/pingping99/keyantong-autosign.git
cd keyantong-autosign
```

2. 可选将环境变量写入 `.env` 文件（Docker Compose 会自动加载）：

```dotenv
ABLESCI_EMAIL=you@example.com
ABLESCI_PASSWORD=secret
CHECK_INTERVAL=30m           # 每次签到检查间隔
DYNAMIC_WINDOW_START=08:00   # 签到窗口起始时间
DYNAMIC_WINDOW_END=18:00     # 签到窗口结束时间
DYNAMIC_WINDOW_SPAN=45m      # 签到窗口时长
TZ=Asia/Shanghai
DATA_DIR=/app/data           # 容器内部日志/状态目录
```

3. 启动容器：

```bash
docker compose up -d --build
```

4. 可选查看日志和状态：

```bash
docker compose logs -f ablesci-sign
docker compose exec ablesci-sign cat /app/data/sign.log
```

5. 停止并清理：

```bash
docker compose down
```

### 环境变量说明

- `ABLESCI_EMAIL`：AbleSci 登录邮箱（必填）
- `ABLESCI_PASSWORD`：AbleSci 登录密码（必填）
- `CHECK_INTERVAL`：签到检查频率，支持 Go `time.Duration` 表达式（默认 `30m`）
- `DYNAMIC_WINDOW_START`：动态签到窗口的起始时间（格式 `HH:MM`，默认 `08:00`）
- `DYNAMIC_WINDOW_END`：动态签到窗口的结束时间（格式 `HH:MM`，默认 `18:00`）
- `DYNAMIC_WINDOW_SPAN`：每日动态窗口的时长（支持 Go `time.Duration` 表达式，默认 `45m`）
- `TZ`：运行时时区（默认 `Asia/Shanghai`）
- `DATA_DIR`：日志和状态存放路径（默认 `./data`），可通过 `docker volume` 挂载到宿主机便于持久化

**动态签到窗口说明**：
- 程序会在每天的 `DYNAMIC_WINDOW_START` 至 `DYNAMIC_WINDOW_END` 范围内随机生成一个签到窗口
- 窗口时长由 `DYNAMIC_WINDOW_SPAN` 指定（例如 45 分钟）
- 每天的窗口位置随机，避免固定时间签到被检测
- 同一天内窗口保持不变（持久化到状态文件）
- 在窗口内，系统会随机选择一个具体时间进行签到，进一步增强随机性

## 依赖

- Go 1.21+
- net/http - HTTP 请求
- cookiejar - Cookie 管理

## 许可证

MIT License
