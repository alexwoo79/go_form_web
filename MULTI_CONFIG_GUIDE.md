# 多配置文件合并功能 - 使用说明

## 📋 功能概述

系统现已支持多配置文件合并功能，可以将不同业务模块的表单定义分散在多个 YAML 文件中，启动时自动加载并合并到统一配置，所有数据存储在同一个数据库中。

## 🎯 核心优势

### 1. **模块化管理**
- 不同业务部门的表单独立维护
- 配置文件职责清晰，易于协作
- 可以独立版本控制和回滚

### 2. **统一数据存储**
- 所有数据在同一个数据库中，便于关联查询
- 一个管理后台查看所有表单数据
- 跨表单统计分析更容易

### 3. **灵活扩展**
- 新增业务表单只需添加新配置文件
- 不影响现有配置
- 支持通配符批量加载

## 📁 文件结构

```
go-web/
├── config.yaml              # 主配置文件（服务器 + 数据库 + includes）
├── marketing_forms.yaml     # 市场营销表单
├── hr_forms.yaml           # 人力资源表单
├── survey_forms.yaml       # 调研问卷表单
└── data/
    └── data.db             # 统一数据库文件
```

## 🔧 配置示例

### 主配置文件 (config.yaml)

```yaml
server:
  port: 8080
  host: "localhost"

database:
  path: "data/data.db"
  type: "sqlite"

# 包含的其他配置文件（支持通配符）
includes:
  - "marketing_forms.yaml"
  - "hr_forms.yaml"
  - "survey_forms.yaml"
  # 也可以使用通配符：
  # - "*.yaml"  # 加载所有 yaml 文件
  # - "forms/*.yaml"  # 加载 forms 目录下所有文件

# 主配置中的表单（核心表单）
forms:
  - name: "user_registration"
    title: "用户注册表单"
    # ... 字段定义
```

### 子配置文件 (marketing_forms.yaml)

```yaml
# 只需要定义 forms 列表
forms:
  - name: "feedback"
    title: "用户反馈表单"
    description: "请告诉我们您的使用体验和建议"
    data_directory: "data/feedback"
    model:
      table_name: "marketing_feedback"
    fields:
      - name: "name"
        label: "姓名"
        type: "text"
        required: true
      # ... 更多字段
```

## 💡 使用场景

### 场景 1：按部门划分
```yaml
includes:
  - "marketing/*.yaml"     # 市场部表单
  - "hr/*.yaml"           # 人力资源部表单
  - "product/*.yaml"      # 产品部表单
  - "support/*.yaml"      # 客服部表单
```

### 场景 2：按业务线划分
```yaml
includes:
  - "core_forms.yaml"      # 核心业务表单
  - "campaign_*.yaml"      # 营销活动表单（多个文件）
  - "survey_*.yaml"        # 调研问卷表单
```

### 场景 3：按环境划分
```yaml
includes:
  - "common_forms.yaml"    # 通用表单
  - "dev_forms.yaml"       # 开发环境专用
  # 或
  - "prod_forms.yaml"      # 生产环境专用
```

## 🔍 配置来源追踪

系统会自动记录每个表单来自哪个配置文件，在管理后台可以查看：

```go
// FormConfig 结构体新增字段
type FormConfig struct {
    Name          string
    Title         string
    // ... 其他字段
    ConfigSource  string  // 记录配置来源文件名
}
```

## ⚠️ 注意事项

### 1. **表单名称唯一性**
确保不同配置文件中的 `name` 字段不重复，否则会发生冲突。

### 2. **表名命名规范**
建议使用前缀区分业务模块：
```yaml
# 市场部
table_name: "marketing_feedback"
table_name: "marketing_newsletter"

# 人力资源部
table_name: "hr_job_application"
table_name: "hr_employee_feedback"

# 调研问卷
table_name: "survey_customer_satisfaction"
table_name: "survey_product_research"
```

### 3. **加载顺序**
配置文件按 `includes` 列表顺序依次加载，后加载的配置会追加到表单列表末尾。

### 4. **错误处理**
如果某个配置文件加载失败，系统会打印警告日志但不会中断启动，继续使用其他成功的配置。

## 🧪 测试验证

### 检查配置文件是否加载
```bash
# 启动服务器，查看日志
./bin/go-web ./config.yaml

# 应该看到类似输出：
# ✅ 已加载配置文件：marketing_forms.yaml (2 个表单)
# ✅ 已加载配置文件：hr_forms.yaml (2 个表单)
# ✅ 已加载配置文件：survey_forms.yaml (2 个表单)
```

### 验证首页表单列表
```bash
curl http://localhost:8080/ | grep "<h3>"

# 输出应包含所有配置文件中的表单
```

### 验证数据库表创建
```bash
sqlite3 data/data.db ".tables"

# 应该看到所有表：
# user_registration
# event_registration
# marketing_feedback
# marketing_newsletter
# hr_job_application
# hr_employee_feedback
# survey_customer_satisfaction
# survey_product_research
```

## 📊 性能影响

- **启动时间**: 每增加一个配置文件增加约 10-20ms
- **内存占用**: 配置对象大小与单个大配置相同
- **运行性能**: 无影响，配置只在启动时加载一次

## 🚀 最佳实践

1. **保持配置精简**: 每个配置文件专注于一个业务领域
2. **使用有意义的命名**: 表名、字段名加上业务前缀
3. **定期清理**: 删除不再使用的配置文件
4. **文档化**: 在每个配置文件顶部添加注释说明用途
5. **版本控制**: 将配置文件纳入 Git 管理，记录变更历史

## 🎯 未来扩展

后续可以考虑的功能：
- [ ] 配置热重载（无需重启服务器）
- [ ] 配置验证工具（检查语法和逻辑错误）
- [ ] 配置差异对比（git diff 可视化）
- [ ] 表单导入导出（JSON/YAML格式）

---

**更新时间**: 2026-03-25  
**版本**: v2.0