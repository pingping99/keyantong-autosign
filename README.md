# AbleSci 自动签到脚本

基于 Go 语言开发的科研通（AbleSci.com）自动签到脚本。项目已在 GitHub 开源：<https://github.com/pingping99/keyantong-autosign>。

## 功能特性

- ✅ 自动登录
- ✅ 自动签到
- ✅ 支持 CSRF 令牌验证
- ✅ Cookie 会话管理
- ✅ 详细的签到结果显示
- ✅ 首次使用自动登录签到
- ✅ 登录状态检查与自动重登录

## 项目结构

```
keyantong/
├── main.go              # 主程序入口
├── go.mod               # Go 模块依赖
├── .env.example         # 环境变量示例
├── client/
│   └── client.go        # HTTP 客户端封装（含 Cookie 管理）
├── config/
│   └── config.go        # 配置加载（环境变量）
├── domain/
│   ├── account.go       # 账号实体
│   └── state.go         # 签到状态实体
├── scheduler/
│   └── window.go        # 时间窗工具函数
├── service/
│   └── sign.go          # AbleSci API 调用（登录、签到）
├── signer/
│   └── signer.go        # 签到流程编排
├── store/
│   ├── state_store.go   # 状态存储接口
│   └── file_store.go    # 文件系统实现
└── interface.md         # 接口文档
```

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 配置账号

使用环境变量配置您的账号信息：

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

- 请妥善保管您的账号密码，建议使用环境变量而不是硬编码
- 建议配合定时任务（如 cron、Windows 任务计划程序）实现每日自动签到
- 本脚本仅供学习交流使用
- 首次运行时会自动执行登录与签到
- 签到状态保存在 `data/state.json`
- 程序运行时会自动检查登录状态，如果会话失效会自动重新登录

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
- `RETRY_INTERVAL`：窗口内签到失败后的最小重试间隔（默认 `10m`）
- `FORCE_SIGN_ON_START`：程序启动时是否立即签到（默认 `true`）
- `TZ`：运行时时区（默认 `Asia/Shanghai`）
- `DATA_DIR`：日志和状态存放路径（默认 `./data`），可通过 `docker volume` 挂载到宿主机便于持久化

## 依赖

- Go 1.21+
- 仅使用标准库（`net/http`、`net/http/cookiejar`），无需第三方依赖

## 许可证

MIT License
