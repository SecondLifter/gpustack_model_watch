# GPUSTACK 监控工具

一个用于监控和管理 GPUSTACK 模型状态的工具。主要功能是自动检测并删除错误状态的模型实例。

## 功能特性

- 自动监控模型实例状态
- 自动检测并删除错误状态的模型实例
- 支持自动登录和会话维护
- 具有重试机制和错误处理
- 优雅的退出处理

## 构建和运行

### 构建
```bash
go build -o model-watcher
```

### 运行
```bash
./model-watcher -url="http://your-api-url" -username="your-username" -password="your-password"
```

### 命令行参数
- `-url`: API 基础 URL (默认: "http://127.0.0.1")
- `-username`: 登录用户名 (默认: "admin")
- `-password`: 登录密码 (默认: "TKn2QhA9wf7wy")

### 登录接口
```
POST /auth/login
Content-Type: application/x-www-form-urlencoded

参数：
- username: 用户名
- password: 密码

返回：
- 200: 登录成功，返回 token
- 400: 登录失败
```



## 错误处理

- 登录失败：自动重试登录
- 会话过期：自动重新登录
- API 错误：记录错误并继续监控
- 网络错误：自动重试

## 监控逻辑

1. 程序启动时进行首次登录
2. 每 30 秒检查一次所有模型状态
3. 发现错误状态的模型实例时自动删除
4. 遇到认证错误时自动重新登录
5. 最多重试 3 次，每次重试间隔 5 秒

## 注意事项

1. 确保提供正确的 API URL 和登录凭据
2. 程序会持续运行直到收到中断信号
3. 所有操作都会记录到日志中
4. 删除操作不可恢复，请谨慎使用
```