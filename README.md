# EasyC2 - 加密C2通信框架

## 项目概述

EasyC2是一个用于安全研究的C2（Command and Control）框架，实现了类似CobaltStrike的加密通信机制。该项目包含三个主要组件：

- **Server**: 服务端，负责消息转发和加密通信管理
- **Client**: 客户端（木马），支持跨平台命令执行和截图
- **Controller**: 控制端，提供图形化界面进行远程控制

## 主要特性

### 🔐 加密通信
- **RSA+AES混合加密**: 使用RSA加密传输AES密钥，后续通信使用AES加密
- **Beacon连接协议**: 类似CobaltStrike的beacon连接过程
- **双向握手**: 确保连接安全性和可靠性
- **心跳机制**: 保持连接活跃状态

### 🌐 跨平台支持
- **Windows**: 完整支持，包括截图和命令执行
- **Linux**: 支持命令执行和截图（需要X11环境）
- **macOS**: 支持命令执行和截图（需要图形环境）

### 🛡️ 安全特性
- **密钥动态生成**: 每次连接使用新的RSA密钥对
- **AES-GCM模式**: 提供认证加密
- **消息类型验证**: 防止协议混淆攻击
- **连接状态管理**: 自动重连和错误处理

## 架构设计

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  Controller │    │    Server   │    │    Client   │
│   (控制端)   │◄──►│   (服务端)   │◄──►│   (客户端)   │
│             │    │             │    │             │
│  GUI界面     │    │  消息转发    │    │ 命令执行     │
│ 命令发送     │    │ 加密管理      │    │ 截图功能     │
└─────────────┘    └─────────────┘    └─────────────┘
```

## 通信协议

### 消息类型
- `0x01`: 握手消息
- `0x02`: 握手确认
- `0x03`: 密钥交换
- `0x04`: 密钥交换确认
- `0x05`: 数据消息
- `0x06`: 数据确认
- `0x07`: 心跳消息
- `0x08`: 心跳确认

### 连接流程
1. **TCP连接建立**
2. **握手过程**: 客户端发送握手消息，服务端确认
3. **密钥交换**: 
   - 客户端生成RSA密钥对，发送公钥
   - 服务端生成AES密钥，用RSA公钥加密后发送
   - 客户端解密获得AES密钥
4. **加密通信**: 后续所有通信使用AES-GCM加密
5. **心跳维护**: 定期发送心跳保持连接

## 使用方法

### 1. 编译项目

```bash
# 编译服务端
cd server
go build -o server server.go

# 编译客户端
cd ../client
go build -o client client.go

# 编译控制端
cd ../controller
go build -o controller controller.go
```

### 2. 运行服务端

```bash
cd server
./server
```

服务端将在以下端口监听：
- 80端口：客户端连接
- 50050端口：控制端连接

### 3. 运行客户端

```bash
cd client
./client
```

客户端将自动连接到服务端并建立加密通信。

### 4. 运行控制端

```bash
cd controller
./controller
```

控制端将启动图形界面，连接到服务端后可以进行远程控制。

## 支持的命令

### 系统命令
- `shell <command>`: 执行系统命令
- `screenshot`: 截取屏幕截图

### 快捷命令
- IP配置: `shell ipconfig` (Windows) / `shell ifconfig` (Linux/macOS)
- 当前用户: `shell whoami`
- 列出文件: `shell dir` (Windows) / `shell ls` (Linux/macOS)
- 进程列表: `shell tasklist` (Windows) / `shell ps` (Linux/macOS)
- 系统信息: `shell systeminfo` (Windows) / `shell uname -a` (Linux/macOS)

## 安全注意事项

⚠️ **重要提醒**：
- 本项目仅用于安全研究和教育目的
- 请勿在未授权的系统上使用
- 使用者需承担相应的法律责任
- 建议在隔离的测试环境中运行

## 技术细节

### 加密实现
- **RSA**: 2048位密钥，用于密钥交换
- **AES**: 256位密钥，GCM模式，用于数据加密
- **随机数生成**: 使用crypto/rand确保安全性

### 跨平台兼容性
- **命令执行**: 根据操作系统自动选择shell
- **截图功能**: 使用kbinani/screenshot库
- **文件路径**: 自动适配不同操作系统的临时目录

### 错误处理
- **连接重试**: 自动重连机制
- **超时控制**: 命令执行30秒超时
- **错误恢复**: 优雅的错误处理和资源清理

## 依赖项

### 服务端
- Go 1.21+
- 标准库：crypto, net, sync

### 客户端
- Go 1.21+
- github.com/kbinani/screenshot
- 标准库：crypto, net, os/exec

### 控制端
- Go 1.21+
- fyne.io/fyne/v2
- golang.org/x/text/encoding/simplifiedchinese

## 许可证

本项目仅用于安全研究和教育目的。使用者需遵守当地法律法规。

## 贡献

欢迎提交Issue和Pull Request来改进项目。

## 更新日志

### v0.2
- ✨ 新增RSA+AES加密通信
- ✨ 实现Beacon连接协议
- ✨ 支持跨平台兼容性
- ✨ 添加心跳机制
- ✨ 改进错误处理和重连机制
- 🐛 修复Windows特定代码的兼容性问题 
