#!/bin/bash

# Go Web Form System - 初始化脚本

echo "=== Go Web Form System 初始化脚本 ==="
echo ""

# 创建必要目录
echo "创建目录结构..."
mkdir -p data
mkdir -p generated
mkdir -p ui/templates
mkdir -p cmd/server
mkdir -p cmd/generate
mkdir -p internal/config
mkdir -p internal/handler
mkdir -p internal/models
mkdir -p internal/utils
mkdir -p ui/static/css
mkdir -p ui/static/js
mkdir -p bin

# 创建配置文件（如果不存在）
if [ ! -f "config.yaml" ]; then
    echo "创建示例配置文件..."
    cat > config.yaml << 'EOF'
server:
  port: 8080
  host: "localhost"

database:
  path: "data/data.db"
  type: "sqlite"

forms:
  - name: "user_registration"
    title: "用户注册表单"
    description: "欢迎注册我们的服务，请填写以下信息"
    data_directory: "data/user_registration"
    model:
      table_name: "user_registration"
    fields:
      - name: "username"
        label: "用户名"
        type: "text"
        placeholder: "请输入用户名"
        required: true
      - name: "email"
        label: "邮箱"
        type: "email"
        placeholder: "请输入邮箱地址"
        required: true
      - name: "phone"
        label: "手机号"
        type: "tel"
        placeholder: "请输入手机号"
        required: false
      - name: "age"
        label: "年龄"
        type: "number"
        placeholder: "请输入年龄"
        required: false
        min: 1
        max: 150
      - name: "gender"
        label: "性别"
        type: "select"
        options:
          - "男"
          - "女"
          - "其他"
        required: false
EOF
    echo "✅ 配置文件已创建: config.yaml"
else
    echo "✅ 配置文件已存在: config.yaml"
fi

# 创建数据库目录
mkdir -p data

echo ""
echo "初始化完成！"
echo ""
echo "下一步操作："
echo "1. 运行 'go mod download' 下载依赖"
echo "2. 运行 'go run cmd/server/main.go' 启动服务器"
echo "3. 访问 http://localhost:8080 查看表单"
echo ""
echo "目录结构:"
echo "  data/           # 数据文件"
echo "  generated/      # 生成的表单页面"
echo "  ui/templates/   # HTML 模板"
echo ""
echo "配置文件位置: config.yaml"
echo "数据库位置: data/data.db"
