# AbleSci 自动签到（keyantong-autosign）

科研通（AbleSci.com）单账户自动签到工具。程序在配置的工作时间内生成每日随机签到时间，处理会话过期，并持久化本地调度状态和日志。

## 主要特性

- **单账户运行**：只接受一个 AbleSci 邮箱和密码。
- **账户状态隔离**：`state.json` 保存账户指纹；更换账户并复用数据目录时，旧状态会被隔离，不会阻止新账户签到。
- **随机签到时间**：每日在工作时间窗口内生成随机签到时间。
- **工作时间保护**：请求前随机等待不会超过工作时间结束点，等待后会再次校验时间。
- **自动重新登录**：仅在明确检测到会话失效时重新登录并重试一次。
- **严格结果判定**：只有业务码 `0` 或 `1` 才会写入成功状态。
- **完成时间节流**：失败重试间隔从请求实际完成时间开始计算。
- **状态保护**：损坏状态和其他账户的状态都会被隔离；Unix 使用临时文件重命名替换，Windows 使用可恢复的备份替换。
- **健康检查**：默认只监听 `127.0.0.1`，失败时返回 HTTP 503，响应不包含上游详细错误。
- **零外部 Go 依赖**：仅使用 Go 标准库。

## 快速开始

### 环境变量

```bash
export ABLESCI_EMAIL=your@example.com
export ABLESCI_PASSWORD=your_password
go run .
```

邮箱和密码必须同时来自环境变量。只设置其中一项时程序会直接报错，不会与 YAML 中的另一项拼接。

### YAML 配置

```bash
cp config.yaml.example config.yaml
# 编辑 email 和 password
go run .
```

配置文件搜索顺序：

1. `CONFIG_FILE` 指定的文件；
2. `config.yaml`；
3. `config.yml`；
4. `data/config.yaml`；
5. `data/config.yml`。

环境变量优先级高于配置文件，但账户凭证必须整体来自同一个来源。

## 配置项

| 环境变量 | YAML 键名 | 默认值 | 说明 |
|---|---|---:|---|
| `ABLESCI_EMAIL` | `email` | 无 | 必填，单账户邮箱 |
| `ABLESCI_PASSWORD` | `password` | 无 | 必填，单账户密码 |
| `DATA_DIR` | `data_dir` | `./data` | 状态和日志目录 |
| `CHECK_INTERVAL` | `check_interval` | `30m` | 正常检查间隔 |
| `RETRY_INTERVAL` | `retry_interval` | `10m` | 从上次尝试完成后计算的重试间隔 |
| `SIGN_JITTER_MAX` | `sign_jitter_max` | `5m` | 请求前最大随机等待；`0s` 表示关闭 |
| `FORCE_SIGN_ON_START` | `force_sign_on_start` | `false` | 启动时立即尝试，仍受工作时间、状态和节流限制 |
| `TZ` | `timezone` | `Asia/Shanghai` | 调度时区 |
| `EARLY_HOUR_THRESHOLD` | `early_hour_threshold` | `8` | 工作时间开始小时，范围 0–23 |
| `LATE_HOUR_THRESHOLD` | `late_hour_threshold` | `22` | 工作时间结束小时，范围 1–24 |
| `HEALTH_CHECK_HOST` | `health_check_host` | `127.0.0.1` | 健康接口监听地址 |
| `HEALTH_CHECK_PORT` | `health_check_port` | `8080` | 健康接口端口 |
| `API_BASE_URL` | `api.base_url` | AbleSci 地址 | API 基础地址 |
| `API_LOGIN_PATH` | `api.login_path` | `/site/login` | 登录路径 |
| `API_SIGN_PATH` | `api.sign_path` | `/user/sign` | 签到路径 |

时间间隔使用 Go duration 格式，例如 `30m`、`1h`、`2h30m`。非法配置会使程序直接退出。

### YAML 语法范围

为保持零外部依赖，解析器只支持本项目需要的标量键和一层 `api:` 配置。密码包含 `#`、冒号或前后空格时应使用引号：

```yaml
password: 'p@ss: word #1'
```

旧的 `accounts:` 数组和 `data/accounts.json` 已移除；发现 `accounts:` 时程序会明确报错。

## Docker Compose

```bash
cp .env.example .env
mkdir -p data
# 编辑 .env 中的账户凭证、PUID 和 PGID
docker compose up -d --build
docker compose logs -f keyantong-autosign
```

Linux 下建议使用当前用户 ID，避免容器以 root 运行并确保 `./data` 可写：

```bash
printf 'PUID=%s\nPGID=%s\n' "$(id -u)" "$(id -g)"
```

将输出值写入 `.env`。已有数据目录权限不正确时，可执行：

```bash
sudo chown -R "$(id -u):$(id -g)" data
```

Docker 构建只复制 `main.go`、`config/`、`core/` 和 `go.mod`；`.dockerignore` 还会排除 `.env`、本地配置、日志和 `data/`，防止凭证进入构建上下文。

## 健康检查

默认地址：

```text
http://127.0.0.1:8080/health
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

- `pending` 或 `success`：HTTP 200；
- `failed`：HTTP 503；
- `last_error` 只返回错误类别，例如 `网络错误`，不会暴露上游响应体或账户信息。

默认仅监听本机。确需从其他主机访问时，显式配置：

```bash
HEALTH_CHECK_HOST=0.0.0.0
```

同时应通过防火墙或反向代理限制访问。

## 状态文件

运行状态保存在：

```text
data/state.json
```

状态包含当前邮箱的 SHA-256 截断指纹，不保存邮箱和密码。更换账户但继续使用原数据目录时：

1. 原 `state.json` 会被移动为 `state.json.account-mismatch-<时间戳>`；
2. 新账户从空状态开始；
3. 旧账户的状态不会被当作新账户的签到结果。

损坏状态会被移动为：

```text
data/state.json.corrupt-<时间戳>
```

Unix 平台使用同目录临时文件重命名替换；Windows 使用带备份回滚的安全替换。Windows 文件替换不宣称具备 Unix 相同的原子语义。

## 从多账户版本迁移

升级后只保留一个账户：

1. 删除配置中的 `accounts:` 数组；
2. 将保留账户写为顶层 `email` 和 `password`，或同时设置两个环境变量；
3. 不再使用 `data/accounts.json`；
4. 程序会根据邮箱查找旧的 `state_<账户ID>.json` 并迁移到 `state.json`；
5. 迁移状态会写入账户指纹，但保持为未确认版本；首次有效检查仍会向服务器确认。

## 签到流程

1. 加载并严格校验单账户配置；
2. 加载与当前账户指纹匹配的状态；
3. 必要时生成当天随机签到时间；
4. 到达执行时间后记录计划时间；
5. 将随机等待裁剪到工作时间结束前，并在等待后再次检查时间；
6. 记录实际请求时间并发起签到；
7. 仅在会话失效时重新登录并重试一次；
8. 业务码 `0` 记为成功，业务码 `1` 记为今日已签到；
9. 其他业务码记为失败，不更新 `last_sign_date`；
10. 从实际完成时间开始计算下一次重试间隔。

## 开发检查

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
go build ./...
```

CI 覆盖：

- Go 1.21 和 Go 1.23；
- Ubuntu 和 Windows；
- Linux Go 1.23 额外运行 race detector。

最低 Go 版本为 1.21；Docker 构建使用 Go 1.23。

## 许可

MIT License
