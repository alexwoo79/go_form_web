package models

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

type Database struct {
	db *sql.DB
}

type UserRecord struct {
	ID           int
	Username     string
	PasswordHash string
	Role         string
	CreatedAt    string
}

type ShareLinkRecord struct {
	Token     string
	FormName  string
	CreatedBy int
	CreatedAt string
}

func NewDatabase(dbPath, dbType string) (*Database, error) {
	// Ensure dbType is "sqlite"
	if dbType != "sqlite" && dbType != "sqlite3" {
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

// columnExists 检查表中的列是否存在
func (d *Database) columnExists(tableName, columnName string) bool {
	query := `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`
	var count int
	err := d.db.QueryRow(query, tableName, columnName).Scan(&count)
	return err == nil && count > 0
}

// getTableColumns 获取表的所有列名
func (d *Database) getTableColumns(tableName string) []string {
	rows, err := d.db.Query(`SELECT name FROM pragma_table_info(?)`, tableName)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			continue
		}
		columns = append(columns, col)
	}
	return columns
}

func (d *Database) TableExists(tableName string) bool {
	query := `SELECT COUNT(name) FROM sqlite_master WHERE type='table' AND name=?`
	var count int
	if err := d.db.QueryRow(query, tableName).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func (d *Database) CreateTable(tableName string, fields []FieldInfo) error {
	// 构建动态列结构
	columns := make([]string, 0)
	columns = append(columns, "id INTEGER PRIMARY KEY AUTOINCREMENT")

	// 添加 data 字段用于存储原始 JSON（备份用途）
	columns = append(columns, "data TEXT NOT NULL")

	// 根据字段定义创建列
	for _, field := range fields {
		colType := d.getFieldType(field.Type)
		columns = append(columns, fmt.Sprintf("`%s` %s", field.Name, colType))
	}

	// 添加系统字段
	columns = append(columns, "owner_user_id INTEGER")
	columns = append(columns, "_submitted_at TEXT")
	columns = append(columns, "_ip TEXT")

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (%s)", tableName, strings.Join(columns, ", "))
	_, err := d.db.Exec(query)
	return err
}

func (d *Database) ensureFormSystemColumns(tableName string) error {
	systemCols := map[string]string{
		"owner_user_id": "INTEGER",
		"_submitted_at": "TEXT",
		"_ip":           "TEXT",
	}

	for col, typ := range systemCols {
		if d.columnExists(tableName, col) {
			continue
		}
		query := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s", tableName, col, typ)
		if _, err := d.db.Exec(query); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("添加系统列 %s 失败：%v", col, err)
		}
	}

	return nil
}

// getFieldType 根据表单字段类型返回数据库字段类型
func (d *Database) getFieldType(formFieldType string) string {
	switch formFieldType {
	case "number", "range":
		return "REAL"
	case "date", "time":
		return "TEXT" // SQLite 没有专门的日期类型
	default:
		return "TEXT"
	}
}

// UpdateTableSchema 动态更新表结构（添加新列）
func (d *Database) UpdateTableSchema(tableName string, oldFields []FieldInfo, newFields []FieldInfo) error {
	// 如果表不存在，直接创建
	if !d.TableExists(tableName) {
		return d.CreateTable(tableName, newFields)
	}

	// 获取现有列名
	existingCols := make(map[string]bool)
	for _, f := range oldFields {
		existingCols[f.Name] = true
	}

	// 添加新列
	for _, field := range newFields {
		if !existingCols[field.Name] {
			colType := d.getFieldType(field.Type)
			query := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s", tableName, field.Name, colType)
			if _, err := d.db.Exec(query); err != nil {
				// 如果是重复列的错误，忽略（静默处理）
				if !strings.Contains(err.Error(), "duplicate column name") {
					return fmt.Errorf("添加列 %s 失败：%v", field.Name, err)
				}
			}
		}
	}

	if err := d.ensureFormSystemColumns(tableName); err != nil {
		return err
	}

	return nil
}

func (d *Database) Insert(tableName string, data map[string]interface{}) error {
	// 动态构建列名和占位符
	columns := make([]string, 0)
	values := make([]interface{}, 0)
	placeholders := make([]string, 0)

	// 调试：打印插入信息
	fmt.Printf("💾 准备插入数据到表：%s\n", tableName)
	fmt.Printf("📦 原始数据：%v\n", data)

	// 首先处理 data 字段（存储原始 JSON）
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化 data 失败：%v", err)
	}
	columns = append(columns, "data")
	values = append(values, jsonData)
	placeholders = append(placeholders, "?")

	for key, val := range data {
		// 跳过系统字段（稍后单独处理）
		if key == "owner_user_id" || key == "_submitted_at" || key == "_ip" {
			continue
		}

		// 检查列是否存在
		if !d.columnExists(tableName, key) {
			fmt.Printf("❌ 错误：表 %s 中不存在列 %s\n", tableName, key)
			fmt.Printf("📋 表 %s 的可用列：%v\n", tableName, d.getTableColumns(tableName))
			return fmt.Errorf("表 %s 没有列 %s", tableName, key)
		}

		// 处理数组类型（checkbox 多选）
		if arr, ok := val.([]interface{}); ok {
			// 将数组转换为逗号分隔的字符串
			strArr := make([]string, len(arr))
			for i, v := range arr {
				if s, ok := v.(string); ok {
					strArr[i] = s
				} else {
					strArr[i] = fmt.Sprintf("%v", v)
				}
			}
			val = strings.Join(strArr, ",")
		} else if arr, ok := val.([]string); ok {
			// 处理字符串数组
			val = strings.Join(arr, ",")
		}

		columns = append(columns, fmt.Sprintf("`%s`", key))
		values = append(values, val)
		placeholders = append(placeholders, "?")
	}

	// 添加系统字段
	ownerUserID := 0
	if v, ok := data["owner_user_id"].(float64); ok {
		ownerUserID = int(v)
	} else if v, ok := data["owner_user_id"].(int); ok {
		ownerUserID = v
	}
	submittedAt := ""
	ip := ""
	if v, ok := data["_submitted_at"].(string); ok {
		submittedAt = v
	}
	if v, ok := data["_ip"].(string); ok {
		ip = v
	}

	columns = append(columns, "owner_user_id", "_submitted_at", "_ip")
	values = append(values, ownerUserID, submittedAt, ip)
	placeholders = append(placeholders, "?", "?", "?")

	query := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	fmt.Printf("✅ 执行插入：SQL=%s\n", query)

	_, err = d.db.Exec(query, values...)
	if err != nil {
		fmt.Printf("❌ 插入失败：%v\n", err)
	}
	return err
}

func (d *Database) EnsureUserTable() error {
	query := `CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`
	_, err := d.db.Exec(query)
	return err
}

func (d *Database) CreateUser(username, passwordHash, role string) (int64, error) {
	query := `INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, datetime('now'))`
	res, err := d.db.Exec(query, username, passwordHash, role)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *Database) GetUserByUsername(username string) (*UserRecord, error) {
	query := `SELECT id, username, password_hash, role, created_at FROM users WHERE username = ? LIMIT 1`
	var u UserRecord
	err := d.db.QueryRow(query, username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *Database) GetUserByID(id int) (*UserRecord, error) {
	query := `SELECT id, username, password_hash, role, created_at FROM users WHERE id = ? LIMIT 1`
	var u UserRecord
	err := d.db.QueryRow(query, id).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *Database) CountUsers() (int64, error) {
	query := `SELECT COUNT(*) FROM users`
	var count int64
	err := d.db.QueryRow(query).Scan(&count)
	return count, err
}

func (d *Database) UpdateUserRole(userID int, role string) error {
	query := `UPDATE users SET role = ? WHERE id = ?`
	_, err := d.db.Exec(query, role, userID)
	return err
}

func (d *Database) UpdateUserPassword(userID int, passwordHash string) error {
	query := `UPDATE users SET password_hash = ? WHERE id = ?`
	_, err := d.db.Exec(query, passwordHash, userID)
	return err
}

func (d *Database) ListUsers() ([]UserRecord, error) {
	rows, err := d.db.Query(`SELECT id, username, password_hash, role, created_at FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]UserRecord, 0)
	for rows.Next() {
		var u UserRecord
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

func (d *Database) DeleteUser(userID int) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := d.db.Exec(query, userID)
	return err
}

func (d *Database) Query(tableName string, where string, args ...interface{}) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM `%s`", tableName)
	if where != "" {
		query += " WHERE " + where
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}

		results = append(results, rowMap)
	}

	return results, rows.Err()
}

// ExportToCSV 将表单数据导出为 CSV 文件
func (d *Database) ExportToCSV(tableName string, fields []FieldInfo, outputPath string) error {
	// 构建查询所有字段的 SQL
	columns := make([]string, len(fields))
	for i, f := range fields {
		columns[i] = fmt.Sprintf("`%s`", f.Name)
	}

	query := fmt.Sprintf("SELECT %s, _submitted_at, _ip FROM `%s` ORDER BY _submitted_at DESC",
		strings.Join(columns, ", "), tableName)

	rows, err := d.db.Query(query)
	if err != nil {
		return fmt.Errorf("查询数据失败：%v", err)
	}
	defer rows.Close()

	// 创建 CSV 文件
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建文件失败：%v", err)
	}
	defer file.Close()

	// 写入 BOM 以支持 Excel 正确识别 UTF-8
	file.WriteString("\xef\xbb\xbf")

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头（使用中文标签）
	headers := make([]string, 0)
	for _, f := range fields {
		headers = append(headers, f.Label)
	}
	headers = append(headers, "提交时间", "IP 地址")

	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("写入表头失败：%v", err)
	}

	// 写入数据行
	for rows.Next() {
		values := make([]interface{}, len(fields)+2) // 字段 + 系统字段
		valuePtrs := make([]interface{}, len(fields)+2)
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("扫描行失败：%v", err)
		}

		row := make([]string, len(values))
		for i, val := range values {
			if val == nil {
				row[i] = ""
			} else if b, ok := val.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = fmt.Sprintf("%v", val)
			}
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("写入行失败：%v", err)
		}
	}

	return rows.Err()
}

// GetAllData 获取表单的所有数据（用于 API 返回）
func (d *Database) GetAllData(tableName string, fields []FieldInfo) ([]map[string]interface{}, error) {
	return d.Query(tableName, "", nil)
}

// GetCount 获取表中的数据条数
func (d *Database) GetCount(tableName string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)
	var count int64
	err := d.db.QueryRow(query).Scan(&count)
	return count, err
}

func (d *Database) EnsureShareLinkTable() error {
	query := `CREATE TABLE IF NOT EXISTS form_share_links (
		token TEXT PRIMARY KEY,
		form_name TEXT NOT NULL,
		created_by INTEGER NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`
	_, err := d.db.Exec(query)
	return err
}

func (d *Database) CreateShareLink(token, formName string, createdBy int) error {
	if err := d.EnsureShareLinkTable(); err != nil {
		return err
	}

	query := `INSERT INTO form_share_links (token, form_name, created_by, created_at) VALUES (?, ?, ?, datetime('now'))`
	_, err := d.db.Exec(query, token, formName, createdBy)
	return err
}

func (d *Database) GetShareLink(token string) (*ShareLinkRecord, error) {
	if err := d.EnsureShareLinkTable(); err != nil {
		return nil, err
	}

	query := `SELECT token, form_name, created_by, created_at FROM form_share_links WHERE token = ? LIMIT 1`
	var rec ShareLinkRecord
	err := d.db.QueryRow(query, token).Scan(&rec.Token, &rec.FormName, &rec.CreatedBy, &rec.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}
