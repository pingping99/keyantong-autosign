# AbleSci 自动签到（keyantong-autosign）

科研通（AbleSci.com）单账户自动签到工具。程序在设定的工作时间内生成每日随机签到时间，自动处理会话过期，并将本地状态、日志和健康信息持久化。

## 主要特性

- **单账户运行**：仅接受一个 AbleSci 邮箱和密码，避免账户选择和状态混淆。
- **随机签到时间**：每天在工作时间窗口内生成随机执行时间。
- **自动重新登录**：检测到登录页重定向或未登录页面后，重新登录并重试一次。
- **严格结果判定**：只有签到成功或服务器明确返回“今日已签到”时，才写入成功状态。
- **失败重试节流**：失败后按照 `retry_interval` 控制下一次尝试。
- **状态保护**：状态文件原子写入；损坏文件会被隔离为 `.corrupt-*`。
- **健康检查**：提供 `/health` 接口，失败状态返回 HTTP 503。
- **零外部 Go 依赖**：只使用 Go 标准库。

## 重要说明

本地 `state.json` 只是调度缓存，服务器响应才是签到结果的最终依据。HTTP 200 不等于业务签到成功；未知业务响应码会记录为失败并继续按重试间隔尝试。

## 快速开始

### 环境变量

```bash
export ABLESCI_EMAIL=your@example.com
export ABLESCI_PASSWORD=your_password
go run .
```

### YAML 配置

```bash
cp config.yaml.example config.yaml
# 编辑 email 和 password
go run .
```

程序依次搜索：

1. `CONFIG_FILE` 指定的文件；
2. `config.yaml`；
3. `config.yml`；
4. `data/config.yaml`；
5. `data/config.yml`。

环境变量优先级高于配置文件。

## 配置项

| 环境变量 | YAML 键名 | 默认值 | 说明 |
|---|---|---:|---|
| `ABLESCI_EMAIL` | `email` | 无 | 必填，单个账户邮箱 |
| `ABLESCI_PASSWORD` | `password` | 无 | 必填，单个账户密码 |
| `DATA_DIR` | `data_dir` | `./data` | 状态和日志目录 |
| `CHECK_INTERVAL` | `check_interval` | `30m` | 正常检查间隔 |
| `RETRY_INTERVAL` | `retry_interval` | `10m` | 失败后的最短重试间隔 |
| `SIGN_JITTER_MAX` | `sign_jitter_max` | `5m` | 每次请求前的最大随机等待；设为 `0s` 可关闭 |
| `FORCE_SIGN_ON_START` | `force_sign_on_start` | `false` | 启动后是否立即尝试签到，仍会检查工作时间、已签到状态和重试节流 |
| `TZ` | `timezone` | `Asia/Shanghai` | 调度时区 |
| `EARLY_HOUR_THRESHOLD` | `early_hour_threshold` | `8` | 工作时间开始小时，范围 0–23 |
| `LATE_HOUR_THRESHOLD` | `late_hour_threshold` | `22` | 工作时间结束小时，范围 1–24 |
| `HEALTH_CHECK_PORT` | `health_check_port` | `8080` | 健康检查端口 |
| `API_BASE_URL` | `api.base_url` | AbleSci 地址 | API 基础地址 |
| `API_LOGIN_PATH` | `api.login_path` | `/site/login` | 登录路径 |
| `API_SIGN_PATH` | `api.sign_path` | `/user/sign` | 签到路径 |

时间间隔使用 Go duration 格式，例如 `30m`、`1h`、`2h30m`。配置错误会使程序直接退出，不会静默回退。

### YAML 语法范围

为保持零外部依赖，配置解析器支持本项目示例所需的标量键和一层 `api:` 配置。密码含有 `#`、冒号或前后空格时，应使用单引号或双引号：

```yaml
password: 'p@ss: word #1'
```

旧的 `accounts:` 数组和 `data/accounts.json` 已移除，程序发现 `accounts:` 时会明确报错。

## Docker Compose

```bash
cp .env.example .env
# 编辑 .env 中的账户凭证
docker compose up -d --build
docker compose logs -f keyantong-autosign
```

健康检查在容器内部访问：

```bash
wget -qO- http://127.0.0.1:8080/health
```

示例响应：

```json
{
  "status": "success",
  "last_attempt_at": "2026-08-02T10:00:00+08:00",
  "last_success_at": "2026-08-02T10:00:00+08:00",
  "uptime": "3h12m5s"
}
```

状态为 `pending` 或 `success` 时返回 HTTP 200，最近一次签到失败时返回 HTTP 503。

## 从多账户版本迁移

升级后只保留一个账户：

1. 删除配置中的 `accounts:` 数组；
2. 将保留账户写为顶层 `email` 和 `password`，或使用环境变量；
3. 不再使用 `data/accounts.json`；
4. 程序会根据邮箱自动查找旧的 `state_<账户ID>.json`，并在 `state.json` 不存在时迁移一次；旧文件会保留作为备份；
5. 旧版本状态不直接视为可信成功，升级后的首次有效检查会向服务器重新确认，再写入新版状态版本。

## 签到流程

1. 加载并严格校验单账户配置；
2. 读取 `state.json`，必要时生成当天随机签到时间；
3. 到达随机时间后记录本次尝试并加入随机等待；
4. 发起签到请求；
5. 仅在检测到会话失效时重新登录并重试一次；
6. 业务码 `0` 记为成功，业务码 `1` 记为“今日已签到”；
7. 其他业务码记为失败，不更新 `last_sign_date`；
8. 按 `retry_interval` 等待下一次尝试。

## 项目结构

```text
.
├── main.go
├── config/
│   ├── config.go
│   ├── config_test.go
│   └── yaml.go
├── core/
│   ├── errors.go
│   ├── health.go
│   ├── model.go
│   ├── notification.go
│   ├── service.go
│   ├── signer.go
│   ├── signer_test.go
│   ├── store.go
│   ├── store_test.go
│   └── timeutil.go
├── config.yaml.example
├── docker-compose.yml
├── Dockerfile
└── .github/workflows/ci.yml
```

## 开发检查

```bash
gofmt -w .
go vet ./...
go test -race ./...
go build ./...
```

最低 Go 版本为 1.21；CI 和 Docker 构建使用 Go 1.23。

## 许可

MIT License
