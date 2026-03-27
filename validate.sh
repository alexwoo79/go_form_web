#!/bin/bash

# Go Web Form System - Configuration Validation Script

echo "=== 配置文件验证 ==="
echo ""

CONFIG_FILE="${1:-config.yaml}"

# 检查配置文件是否存在
if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ 配置文件不存在: $CONFIG_FILE"
    exit 1
fi

echo "✅ 配置文件存在: $CONFIG_FILE"

# 检查 YAML 格式（简单的语法检查）
echo ""
echo "检查 YAML 语法..."

# 检查必要的 sections
if grep -q "^server:" "$CONFIG_FILE"; then
    echo "✅ 找到 server 配置"
else
    echo "❌ 缺少 server 配置"
    exit 1
fi

if grep -q "^database:" "$CONFIG_FILE"; then
    echo "✅ 找到 database 配置"
else
    echo "❌ 缺少 database 配置"
    exit 1
fi

if grep -q "^forms:" "$CONFIG_FILE"; then
    echo "✅ 找到 forms 配置"
else
    echo "❌ 缺少 forms 配置"
    exit 1
fi

echo ""
echo "检查表单定义..."

# 统计表单数量
FORM_COUNT=$(grep -c "^  - name:" "$CONFIG_FILE" || echo 0)
echo "✅ 找到 $FORM_COUNT 个表单"

if [ "$FORM_COUNT" -eq 0 ]; then
    echo "⚠️  警告: 没有定义任何表单"
else
    echo "✅ 表单数量充足"
fi

echo ""
echo "检查字段定义..."

# 检查是否有字段定义
if grep -q "^\s*- name:" "$CONFIG_FILE"; then
    FIELD_COUNT=$(grep -c "^\s*- name:" "$CONFIG_FILE" || echo 0)
    echo "✅ 找到 $FIELD_COUNT 个字段"
else
    echo "❌ 缺少字段定义"
    exit 1
fi

echo ""
echo "=== 验证通过! ==="
echo ""
echo "下一步：运行 'go run cmd/server/main.go' 或使用 'init.sh' 初始化项目"
