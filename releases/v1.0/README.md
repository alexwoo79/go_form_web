# Go Web Form v1.0 发布版本

## 文件清单

- **go-web** - Go执行程序（主应用二进制文件）
- **config.yaml** - 主要配置文件
- **config.example.yaml** - 配置文件示例
- **hr_forms.yaml** - HR表单配置
- **marketing_forms.yaml** - 市场部表单配置
- **project_forms.yaml** - 项目表单配置
- **survey_forms.yaml** - 调查表单配置

## 功能特性

### 用户认证系统
- ✅ 用户注册和登录
- ✅ 角色管理（普通用户/管理员）
- ✅ 用户密码修改
- ✅ 管理员密码重置功能

### 表单管理
- ✅ 动态表单生成和提交
- ✅ 表单数据持久化存储
- ✅ 提交记录追踪（含提交者、时间戳、IP地址）

### 管理功能
- ✅ 用户管理页面（用户列表、角色分配）
- ✅ 管理员账户保护（角色不可修改、密码仅自身修改）
- ✅ 表单提交统计
- ✅ 提交详情查看

### 系统特性
- ✅ 前后端分离架构（Vue.js + Go）
- ✅ SQLite数据库
- ✅ 多用户并发支持
- ✅ 响应式UI设计

## 快速开始

### 1. 配置

复制并编辑config.yaml：
```bash
cp config.example.yaml config.yaml
# 编辑config.yaml，根据需要调整设置
```

### 2. 运行

```bash
./go-web
```

应用程序将在配置文件中指定的端口启动（默认为http://localhost:8080）

### 3. 初始访问

- 访问 http://localhost:8080
- 使用注册功能创建用户账户
- 或使用命令行初始化管理员账户

## 部署说明

### 系统要求
- Linux/macOS/Windows
- 可选：Go 1.25+ （仅在需要从源代码构建时）

### 环境变量
- 无特殊环境变量要求
- 所有配置通过config.yaml文件管理

### 数据库
- SQLite 3.x（自动初始化）
- 数据存储在config.yaml配置的db_path位置

## 版本历史

### v1.0 (2026-03-28)
- 初始版本发布
- 完整的用户认证系统
- 表单管理和数据收集
- 管理后台功能
- 多用户并发支持

## 技术栈

- **后端**: Go 1.25.0
- **前端**: Vue 3 + TypeScript
- **数据库**: SQLite
- **路由**: Gorilla Mux
- **构建**: Vite + Go build

## 许可证

MIT License

## 支持

如有问题或建议，请提交issue或联系开发团队。
