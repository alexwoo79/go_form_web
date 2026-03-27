# Go Web 表单系统 - 快速开始指南

## 安装步骤

```bash
# 1. 进入项目目录
cd /root/go-web

# 2. 下载依赖
go mod download

# 3. 初始化项目（可选）
./init.sh

# 4. 运行服务器
go run cmd/server/main.go

# 5. 访问应用
浏览器打开: http://localhost:8080
```

## 使用说明

### 1. 配置表单

编辑 `config.yaml` 文件：

```yaml
forms:
  - name: "contact"
    title: "联系我们"
    description: "填写联系方式"
    fields:
      - name: "name"
        label: "姓名"
        type: "text"
        required: true
```

### 2. 启动服务器

```bash
go run cmd/server/main.go
```

### 3. 访问表单

- 主页: http://localhost:8080
- 填写表单: http://localhost:8080/forms/contact

## 数据存储

数据保存在:
- 数据库: `data/data.db`
- 文件: `data/{form-name}/submit_*.json`

## 高级功能

### 生成静态页面

```bash
go run cmd/generate/main.go -output generated
```

### 使用自定义端口

```bash
go run cmd/server/main.go -port 9090
```

### 查看表单数据

```bash
sqlite3 data/data.db
.tables
SELECT * FROM user_registration;
```

## 常见问题

### Q: 如何添加新表单？
A: 编辑 `config.yaml`，添加新的 form 配置即可。

### Q: 数据保存在哪里？
A: 数据同时保存在 SQLite 数据库和 JSON 文件中。

### Q: 如何修改表单字段？
A: 编辑 `config.yaml`，修改 fields 配置，重启服务器。

## API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | / | 首页 |
| GET | /forms | 表单列表 (JSON) |
| GET | /forms/{name} | 表单页面 |
| POST | /api/submit/{name} | 提交表单 |
