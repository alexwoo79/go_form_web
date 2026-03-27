# Go Web 表单系统 - README

[TOC]

## 📋 概述

一个基于 Go 语言和 SQLite 的 Web 表单系统，可以通过 YAML 文件定义 HTML 表单并收集数据。

## ✨ 特性

- ✅ 通过 YAML 文件定义表单
- ✅ 支持多种表单字段类型
- ✅ 使用 SQLite 数据库存储数据
- ✅ 自动创建数据表
- ✅ 支持文件保存
- ✅ 响应式 UI 设计
- ✅ RESTful API 接口

## 🚀 快速开始

### 安装依赖

```bash
cd go-web
go mod download
```

### 运行服务器

```bash
go run cmd/server/main.go
```

### 访问应用

打开浏览器访问: http://localhost:8080

## 📝 配置文件

配置文件使用 YAML 格式：

```yaml
server:
  port: 8080
  host: "localhost"

database:
  path: "data/data.db"
  type: "sqlite"

forms:
  - name: "user_registration"
    title: "用户注册"
    description: "用户注册表单"
    fields:
      - name: "username"
        label: "用户名"
        type: "text"
        required: true
```

## 📋 表单字段类型

| 类型 | 说明 |
|------|------|
| text | 文本输入框 |
| email | 邮箱输入框 |
| tel | 电话输入框 |
| number | 数字输入框 |
| textarea | 多行文本框 |
| select | 下拉选择框 |
| checkbox | 复选框 |
| radio | 单选框 |
| date | 日期选择器 |
| time | 时间选择器 |

## 📂 项目结构

```
go-web/
├── cmd/
│   ├── server/     # Web 服务器
│   └── generate/   # 表单生成器
├── internal/
│   ├── config/     # 配置管理
│   ├── handler/    # HTTP 处理器
│   ├── models/     # 数据模型
│   └── utils/      # 工具函数
├── ui/
│   └── templates/  # HTML 模板
├── data/           # 数据文件
├── config.yaml     # 配置文件
└── go.mod          # Go 模块
```

## 🛠️ 开发

### 构建项目

```bash
go build -o bin/go-web ./cmd/server
```

### 运行测试

```bash
go test ./...
```

## 📄 许可证

MIT License
# go_form_web
