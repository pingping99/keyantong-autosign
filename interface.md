# AbleSci API 接口文档

## 基础信息

- **基础域名**: `https://www.ablesci.com`
- **请求格式**: `application/x-www-form-urlencoded` (登录), `application/json` (签到响应)
- **认证方式**: Cookie-based session (使用 `_identity-frontend` cookie)

---

## 接口列表

### 1. 获取登录页面

#### 接口信息
- **URL**: `GET https://www.ablesci.com/site/login`
- **用途**: 获取 CSRF Token

#### 请求头
```
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8
Accept-Language: zh-CN,zh;q=0.9,en;q=0.8
```

#### 响应
- HTML 页面包含用于 CSRF 保护的 token
- 格式多样：
  - `<meta name="csrf-token" content="xxx">`
  - `<input type="hidden" name="_csrf" ... value="xxx">`（允许其他属性）
  - `<input type="hidden" id="g_csrf_token" value="xxx">`

---

### 2. 用户登录

#### 接口信息
- **URL**: `POST https://www.ablesci.com/site/login`
- **Content-Type**: `application/x-www-form-urlencoded; charset=UTF-8`

#### 请求头
```
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36
Accept: application/json, text/javascript, */*; q=0.01
Accept-Language: zh-CN,zh;q=0.9,en;q=0.8
Accept-Encoding: gzip, deflate, br, zstd
X-Requested-With: XMLHttpRequest
Content-Type: application/x-www-form-urlencoded; charset=UTF-8
Origin: https://www.ablesci.com
Referer: https://www.ablesci.com/site/login
Sec-CH-UA: "Not(A:Brand";v="8", "Chromium";v="144", "Google Chrome";v="144"
Sec-CH-UA-Mobile: ?0
Sec-CH-UA-Platform: "Windows"
Sec-Fetch-Site: same-origin
Sec-Fetch-Mode: cors
Sec-Fetch-Dest: empty
```

#### 请求参数
| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| _csrf | string | 是 | CSRF Token，从登录页面获取（支持 meta + hidden input）
| email | string | 是 | 用户邮箱
| password | string | 是 | 用户密码
| remember | string | 否 | 记住我，值为 "1"

#### 请求示例
```
_csrf=xxx&LoginForm[email]=user@example.com&LoginForm[password]=password&LoginForm[rememberMe]=1
```

#### 响应示例
**成功 (200 OK):**
```json
{
  "code": 0,
  "msg": "登录成功",
  "data": {...}
}
```

**失败:**
```json
{
  "code": 1,
  "msg": "用户名或密码错误"
}
```

#### 响应 Cookie
登录成功后，服务器会设置以下关键 Cookie：
- `_identity-frontend`: 用户身份标识，用于后续签到请求
- `_csrf`: 新的 CSRF Token
- `advanced-frontend`: 会话标识

---

### 3. 每日签到

#### 接口信息
- **URL**: `GET https://www.ablesci.com/user/sign`
- **认证**: 需要登录后的 Cookie (`_identity-frontend`)

#### 请求头
```
User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36
Accept: application/json, text/javascript, */*; q=0.01
Accept-Language: zh-CN,zh;q=0.9,en;q=0.8
Accept-Encoding: gzip, deflate, br, zstd
X-Requested-With: XMLHttpRequest
Referer: https://www.ablesci.com/
Sec-CH-UA: "Not(A:Brand";v="8", "Chromium";v="144", "Google Chrome";v="144"
Sec-CH-UA-Mobile: ?0
Sec-CH-UA-Platform: "Windows"
Sec-Fetch-Site: same-origin
Sec-Fetch-Mode: cors
Sec-Fetch-Dest: empty
Cookie: _identity-frontend=xxx; _csrf=xxx; advanced-frontend=xxx; ...
```

#### 响应示例
**成功 (200 OK):**
```json
{
  "code": 0,
  "msg": "签到成功，您已连续签到 1 次，本次获得 20 积分。注：本次签到触发低分保护机制，您获得了2倍签到积分。",
  "data": {
    "signcount": 1,
    "signpoint": 20,
    "today_history": "<div>...</div>",
    "is_alert": 1
  }
}
```

**已签到:**
```json
{
  "code": 1,
  "msg": "今天已经签到过了"
}
```

#### 响应字段说明
| 字段名 | 类型 | 说明 |
|--------|------|------|
| code | int | 状态码，0=成功，非0=失败 |
| msg | string | 提示消息 |
| data.signcount | int | 连续签到天数 |
| data.signpoint | int | 本次获得积分 |
| data.today_history | string | 历史上的今天（HTML） |
| data.is_alert | int | 是否显示提示，1=是 |

---

## 错误码说明

| 错误码 | 说明 |
|--------|------|
| 0 | 操作成功 |
| 1 | 操作失败（具体原因见 msg 字段） |

---

## 注意事项

1. **CSRF 保护**: 所有 POST 请求需要携带有效的 CSRF Token
2. **Cookie 管理**: 登录后的 Cookie 需要在后续请求中携带
3. **User-Agent**: 建议使用真实浏览器的 User-Agent
4. **频率限制**: 每天只能签到一次
5. **会话过期**: Cookie 有效期为 30 天 (`Max-Age=2592000`)


## 长期运行与状态记录

- **触发方式**: 通过环境变量配置 `ABLESCI_EMAIL`、`ABLESCI_PASSWORD`、`SIGN_WINDOW_START`、`SIGN_WINDOW_END`、`CHECK_INTERVAL`、`RETRY_INTERVAL`、`FORCE_SIGN_ON_START` 以及 `TZ`。
- **签到窗口**: 默认 08:00 到 09:00，仅在窗口内尝试签到。
- **重试间隔**: 默认 10 分钟，窗口内签到失败后的最小重试间隔，避免频繁请求。
- **启动签到**: 默认启用（`FORCE_SIGN_ON_START=true`），程序启动时立即执行签到（无视时间窗口）；设为 `false` 则禁用。
- **状态存储**: `/data/state_<账号hash>.json` 记录签到状态，包括：
  - `last_sign_date`: 最后成功签到日期
  - `last_attempt_date`: 最后尝试日期
  - `last_attempt_time`: 最后尝试时间（HH:MM）
  - `last_result`: 最后结果（success/failed/skip）
- **日志**: 每次运行会追加到 `/data/sign.log`，日志同时输出到 stdout 便于容器查看。
  - **窗口外**: 仅写入文件日志，不污染控制台
  - **窗口内**: 重要日志（签到成功/失败）输出到控制台 + 文件
  - **节流跳过**: 仅写入文件日志
- **签到条件**: 
  1. 当前时间在签到窗口内
  2. 今天尚未成功签到（`last_sign_date` ≠ 今天）
  3. 距离上次尝试超过重试间隔（节流机制）

## Docker 运行说明
1. 构建镜像：

   - 在 builder 阶段中使用 `go build -o /app/signbot .`，确保只为主包生成单个可执行文件，避免多个包写入非目录路径时出现的构建失败。
   - 生成镜像：
```
docker build -t ablesci-sign .
```
```
2. 运行时：
```bash
docker run -d \
  -e ABLESCI_EMAIL=you@example.com \
  -e ABLESCI_PASSWORD=secret \
  -e CHECK_INTERVAL=30m \
  -e SIGN_WINDOW_START=08:00 \
  -e SIGN_WINDOW_END=08:10 \
  -e TZ=Asia/Shanghai \
  -e DATA_DIR=/app/data \
  -v /host/path/data:/app/data \
  ablesci-sign
```
3. 可选使用 docker compose：
```bash
docker compose up -d
```
compose 默认挂载 `./data` 到 `/app/data`，并复用环境变量（适配 `docker-compose.yml`）。
3. DATA_DIR 下保留 `sign.log` 和 `state.json`，便于检查运行情况并避免重复签到。

---

## 代码架构设计

### 模块划分

项目已完成解耦重构，按照职责分离原则拆分为以下模块：

#### 1. `domain/` - 领域模型
- `account.go`: 账号实体定义，包含邮箱、密码和唯一ID
- `state.go`: 签到状态实体，记录最后签到日期

#### 2. `store/` - 状态存储抽象
- `state_store.go`: 定义 `StateStore` 接口，抽象状态持久化操作
  ```go
  type StateStore interface {
      Load(accountID string) (*domain.SignState, error)
      Save(accountID string, state *domain.SignState) error
  }
  ```
- `file_store.go`: 文件系统实现，将状态保存为 JSON 文件

#### 3. `config/` - 配置管理
- `config.go`: 统一管理环境变量读取、时间窗解析、账号加载等配置逻辑
- 职责：配置初始化与验证，返回 `AppConfig` 结构

#### 4. `scheduler/` - 调度工具
- `window.go`: 时间窗判断与格式化工具函数
  - `IsWithinWindow()`: 判断当前时间是否在签到窗口内
  - `FormatWindow()`: 格式化时间窗为 HH:MM

#### 5. `signer/` - 签到流程编排
- `signer.go`: 定义 `Signer` 接口及实现 `AccountSigner`
  ```go
  type Signer interface {
      AttemptSign(now time.Time) error
      ForceSign(now time.Time) error
  }
  ```
- 职责：封装签到业务逻辑，包括窗口检查、状态管理、登录重试

#### 6. `service/` - HTTP 服务层（已存在）
- `sign.go`: 封装 AbleSci API 调用（登录、签到、CSRF获取）

#### 7. `client/` - HTTP 客户端（已存在）
- `client.go`: 封装 HTTP 客户端与 Cookie 管理

#### 8. `main.go` - 应用入口（重构后）
- 精简为依赖注入（DI）+ 启动逻辑
- 职责：组装各模块、启动定时任务
- 代码行数从 400+ 减少到 100-

### 接口设计原则

1. **依赖倒置**：`signer` 依赖 `StateStore` 接口而非具体实现，便于测试与扩展
2. **单一职责**：每个包专注于单一功能领域
3. **开闭原则**：通过接口扩展新功能，无需修改现有代码

---

## 变更历史

### 2026-01-31（签到逻辑鲁棒性优化）
- **扩展状态结构**：`domain.SignState` 新增字段：
  - `LastAttemptDate`: 最后尝试日期
  - `LastAttemptTime`: 最后尝试时间（HH:MM）
  - `LastResult`: 最后结果（success/failed/skip）
- **签到窗口扩展**：默认窗口从 08:00-08:10 扩展到 08:00-09:00，提供更长的签到时间窗口
- **新增重试间隔机制**：
  - 新增 `RETRY_INTERVAL` 环境变量（默认 10 分钟）
  - 窗口内签到失败后，会在间隔时间后自动重试，避免频繁请求
  - 节流逻辑：同一天内若距离上次尝试不足重试间隔，则跳过本次尝试（仅记录文件日志）
- **新增启动签到开关**：
  - 新增 `FORCE_SIGN_ON_START` 环境变量（默认 true）
  - 设为 `false` 可禁用启动强制签到，仅在窗口内尝试签到
- **日志策略优化**：
  - **窗口外**: 仅写入文件日志，不输出到控制台（减少噪声）
  - **窗口内已签到**: 仅写入文件日志
  - **窗口内节流跳过**: 仅写入文件日志
  - **签到成功/失败**: 输出到控制台 + 文件
- **签到逻辑增强**：
  - `performSignWithRetry()`: 封装带登录重试的签到流程
  - `shouldThrottle()`: 基于时间间隔的节流判断
  - 更细粒度的状态记录，便于诊断问题
- **配置优化**：
  - 新增 `config.TimeLayout` 常量（"15:04"）
  - 新增 `parseBoolWithDefault()` 函数支持布尔值环境变量解析
- **影响范围**：
  - 窗口内签到失败后可自动重试，避免"窗口内未签到"的情况
  - 日志输出更简洁，窗口外不再输出大量无用日志
  - 支持按需禁用启动强制签到
  - 状态文件向后兼容（新增字段为空时不影响旧逻辑）

### 2026-01-30（日志输出优化）
- **窗口外日志静默**：当不在签到窗口时，"不在签到窗口"的日志仅写入 `sign.log` 文件，不输出到控制台
- **实现方式**：在 main.go 中创建 `fileLogger` 专用于文件日志，传递给 `AccountSigner`，在窗口判断处使用
- **影响范围**：
  - 控制台输出更简洁，避免频繁的窗口外检查日志干扰
  - `sign.log` 文件仍保留完整的运行记录，便于排查问题
  - 其他日志（签到成功、失败、错误等）输出行为不变

### 2026-01-29（启动流程优化）
- **移除首次运行检测逻辑**：删除 `isFirstRun()` 函数及相关的 log 文件检查代码
- **启动时强制签到**：程序启动时无条件对所有账号执行一次登录与签到（无视时间窗口限制）
- **优化状态文件不存在提示**：
  - `store/file_store.go`: Load 方法对文件不存在场景返回空状态而非错误，避免首次运行的错误日志噪音
  - `signer/signer.go`: 移除 AttemptSign 中"无法加载状态"的错误日志（因 Load 已处理）
- **日志优化**：ForceSign 日志从"首次运行强制执行"改为"强制执行登录与签到"，更准确反映启动行为
- **影响范围**：
  - 每次启动必定尝试签到一次，适合容器重启或定时任务场景
  - 不再依赖 log 文件判断业务逻辑，代码更简洁

### 2026-01-29（解耦重构）
- **架构重构**：将 main.go 的 400+ 行代码按职责拆分为 8 个模块
- **新增接口抽象**：
  - `store.StateStore`: 状态存储接口
  - `signer.Signer`: 签到流程接口
- **新增包**：
  - `domain`: 领域模型（Account, SignState）
  - `store`: 状态持久化抽象与实现
  - `config`: 配置加载与解析
  - `scheduler`: 时间窗工具函数
  - `signer`: 签到业务流程编排
- **main.go 重构**：精简为依赖注入与启动逻辑，提升可测试性与可维护性

### 2026-01-29（初版）
- 初始版本，记录登录和签到接口
- 基于抓包分析的 Chrome 144.0.0.0 请求
- 更新 CSRF 解析方式：优先读取 meta 标签，其次匹配更通用的 hidden input（含 g_csrf_token）
- 登录接口改用页面当前字段（email/password/remember），保持与前端表单一致
- 修复 Docker 多包构建失败，在 builder 阶段改用 `go build -o /app/signbot .`，保证只生成一个可执行文件
- **新增多账号支持**：通过 `data/accounts.json` 配置多个账号，每个账号状态独立管理
- **新增登录状态检查**：每次签到前检查返回结果，如需要登录则自动重新登录后再签到
- 状态文件改为按账号独立保存（`state_<账号hash>.json`），避免多账号状态冲突
