# AbleSci 自动签到脚本

基于 Go 语言开发的科研通（AbleSci.com）自动签到脚本。

## 功能特性

- ✅ 自动登录
- ✅ 自动签到
- ✅ 支持 CSRF 令牌验证
- ✅ Cookie 会话管理
- ✅ 详细的签到结果显示

## 项目结构

```
keyantong/
├── main.go              # 主程序入口
├── go.mod               # Go 模块依赖
├── config.yaml          # 配置文件（需自行创建）
├── config.yaml.example  # 配置文件示例
├── client/
│   └── client.go        # HTTP 客户端封装
├── service/
│   └── sign.go          # 登录和签到业务逻辑
└── interface.md         # 接口文档
```

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 配置账号

复制配置文件模板并填写您的账号信息：

```bash
cp config.yaml.example config.yaml
```

编辑 `config.yaml`：

```yaml
email: "your_email@example.com"
password: "your_password"
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

- 请妥善保管 `config.yaml` 文件，不要提交到版本控制系统
- 建议配合定时任务（如 cron、Windows 任务计划程序）实现每日自动签到
- 本脚本仅供学习交流使用

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

## 技术栈

- Go 1.21+
- net/http - HTTP 请求
- cookiejar - Cookie 管理
- gopkg.in/yaml.v3 - YAML 配置解析

## 许可证

MIT License
