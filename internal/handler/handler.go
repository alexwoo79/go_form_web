package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-web/internal/models"
	"go-web/ui"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl := template.Must(template.ParseFS(ui.Templates, "templates/"+name))
	tmpl.Execute(w, data)
}

// User 用户模型
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"`    // 不序列化到 JSON
	Role     string `json:"role"` // "admin" 或 "user"
}

// Session 会话管理
type Session struct {
	ID        string
	UserID    int
	Username  string
	Role      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionManager 会话管理器
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

func (sm *SessionManager) CreateSession(username string, userID int, role string) string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 生成 session ID
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%d%s", username, userID, time.Now().String())))
	sessionID := hex.EncodeToString(hash[:])

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		Username:  username,
		Role:      role,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24 小时过期
	}

	sm.sessions[sessionID] = session
	return sessionID
}

func (sm *SessionManager) GetSession(sessionID string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists || time.Now().After(session.ExpiresAt) {
		return nil
	}
	return session
}

func (sm *SessionManager) DeleteSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionID)
}

// 清理过期会话
func (sm *SessionManager) CleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for id, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			delete(sm.sessions, id)
		}
	}
}

type Handler struct {
	db         *models.Database
	formMap    map[string]FormInfo
	sessionMgr *SessionManager
	adminUsers map[string]string // username -> password hash
}

type FormInfo struct {
	Name          string
	Title         string
	Description   string
	DataDirectory string
	Model         struct {
		TableName string
	}
	Fields      []FieldInfo
	FileModTime int64 // 配置文件修改时间戳
}

type FieldInfo struct {
	Name        string
	Label       string
	Type        string
	Placeholder string
	Required    bool
	Options     []string
	Min         *float64
	Max         *float64
}

func New(db *models.Database, formConfigs []FormInfo) *Handler {
	formMap := make(map[string]FormInfo)
	for _, fi := range formConfigs {
		formMap[fi.Name] = fi
	}

	h := &Handler{
		db:         db,
		formMap:    formMap,
		sessionMgr: NewSessionManager(),
		adminUsers: make(map[string]string),
	}

	// 初始化默认管理员账户 (admin/admin123)
	defaultPassword := hashPassword("admin123")
	h.adminUsers["admin"] = defaultPassword

	// 定期清理过期会话（每 10 分钟）
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			h.sessionMgr.CleanupExpiredSessions()
		}
	}()

	return h
}

// hashPassword 对密码进行哈希处理
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// verifyPassword 验证密码
func verifyPassword(password, hash string) bool {
	return hashPassword(password) == hash
}

// RequireAdmin 检查用户是否已登录且为管理员（中间件）
func (h *Handler) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		session := h.sessionMgr.GetSession(cookie.Value)
		if session == nil || session.Role != "admin" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next(w, r)
	}
}

// isLoggedIn 检查用户是否已登录
func (h *Handler) isLoggedIn(r *http.Request) bool {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return false
	}

	session := h.sessionMgr.GetSession(cookie.Value)
	return session != nil && !time.Now().After(session.ExpiresAt)
}

// getCurrentUser 获取当前登录用户
func (h *Handler) getCurrentUser(r *http.Request) *Session {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil
	}

	return h.sessionMgr.GetSession(cookie.Value)
}

// convertToModelsField 将 handler.FieldInfo 转换为 models.FieldInfo
func convertToModelsField(fi FieldInfo) models.FieldInfo {
	return models.FieldInfo{
		Name:        fi.Name,
		Label:       fi.Label,
		Type:        fi.Type,
		Placeholder: fi.Placeholder,
		Required:    fi.Required,
		Options:     fi.Options,
		Min:         fi.Min,
		Max:         fi.Max,
	}
}

func (h *Handler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	formList := make([]map[string]interface{}, 0, len(h.formMap))
	for _, fi := range h.formMap {
		formList = append(formList, map[string]interface{}{
			"Name":        fi.Name,
			"Title":       fi.Title,
			"Description": fi.Description,
		})
	}

	renderTemplate(w, "index.html", map[string]interface{}{
		"Forms":    formList,
		"LoggedIn": h.isLoggedIn(r),
	})
}

func (h *Handler) FormListHandler(w http.ResponseWriter, r *http.Request) {
	formList := make([]map[string]interface{}, 0, len(h.formMap))
	for _, fi := range h.formMap {
		formList = append(formList, map[string]interface{}{
			"Name":        fi.Name,
			"Title":       fi.Title,
			"Description": fi.Description,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(formList)
}

func (h *Handler) FormPageHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/forms/"), "/")
	if len(parts) < 1 {
		http.NotFound(w, r)
		return
	}
	formName := parts[0]
	fi, exists := h.formMap[formName]
	if !exists {
		http.NotFound(w, r)
		return
	}

	form := map[string]interface{}{
		"Name":          fi.Name,
		"Title":         fi.Title,
		"Description":   fi.Description,
		"DataDirectory": fi.DataDirectory,
		"Model":         fi.Model,
	}

	// 转换字段
	fields := make([]map[string]interface{}, 0, len(fi.Fields))
	for _, f := range fi.Fields {
		fields = append(fields, map[string]interface{}{
			"Name":        f.Name,
			"Label":       f.Label,
			"Type":        f.Type,
			"Placeholder": f.Placeholder,
			"Required":    f.Required,
			"Options":     f.Options,
			"Min":         f.Min,
			"Max":         f.Max,
		})
	}
	form["Fields"] = fields

	renderTemplate(w, "form.html", form)
}

func (h *Handler) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/submit/"), "/")
	if len(parts) < 1 {
		http.NotFound(w, r)
		return
	}
	formName := parts[0]
	fi, exists := h.formMap[formName]
	if !exists {
		http.NotFound(w, r)
		return
	}

	data := make(map[string]interface{})

	if r.Header.Get("Content-Type") == "application/json" {
		// 解析 JSON body
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "JSON 解析失败：" + err.Error(),
			})
			return
		}
	} else {
		// 解析表单数据
		if err := r.ParseForm(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "表单解析失败",
			})
			return
		}

		// 从表单数据构建 data map
		for _, field := range fi.Fields {
			values := r.Form[string(field.Name)]
			if len(values) == 0 {
				if field.Required {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":  "error",
						"message": fmt.Sprintf("%s 为必填项", field.Label),
					})
					return
				}
				data[field.Name] = nil
				continue
			}

			switch field.Type {
			case "text", "email", "tel", "url", "password":
				data[field.Name] = values[0]
			case "number":
				val, err := strconv.ParseFloat(values[0], 64)
				if err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":  "error",
						"message": fmt.Sprintf("%s 必须是数字", field.Label),
					})
					return
				}
				data[field.Name] = val
			case "textarea":
				data[field.Name] = values[0]
			case "select":
				data[field.Name] = values[0]
			case "checkbox":
				data[field.Name] = values
			case "radio":
				data[field.Name] = values[0]
			case "date":
				data[field.Name] = values[0]
			case "time":
				data[field.Name] = values[0]
			default:
				data[field.Name] = values[0]
			}
		}
	}

	// 验证必填字段 (针对所有提交方式)
	fmt.Printf("🔍 开始验证必填字段，共 %d 个字段\n", len(fi.Fields))
	for _, field := range fi.Fields {
		if field.Required {
			val, exists := data[field.Name]
			fmt.Printf("  字段：%s (类型：%s), 存在：%v, 值：%v (类型：%T)\n", field.Label, field.Type, exists, val, val)

			// 检查是否存在且不为空
			if !exists || val == nil {
				fmt.Printf("    ❌ 验证失败：值不存在或为 nil\n")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  "error",
					"message": fmt.Sprintf("%s 为必填项", field.Label),
				})
				return
			}

			// 根据字段类型进行具体验证
			switch field.Type {
			case "text", "email", "tel", "url", "password", "textarea", "select", "date", "time":
				// 字符串类型检查是否为空
				if str, ok := val.(string); ok && str == "" {
					fmt.Printf("    ❌ 验证失败：空字符串\n")
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":  "error",
						"message": fmt.Sprintf("%s 为必填项", field.Label),
					})
					return
				}
			case "radio":
				// Radio 类型特殊处理
				if str, ok := val.(string); ok && str == "" {
					fmt.Printf("    ❌ 验证失败：空字符串\n")
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":  "error",
						"message": fmt.Sprintf("%s 为必填项", field.Label),
					})
					return
				}
			case "number":
				// 数字类型检查是否为 0 或空
				if num, ok := val.(float64); ok {
					// 注意：0 是有效值，只有 NaN 才无效
					if num != num { // NaN check
						fmt.Printf("    ❌ 验证失败：NaN\n")
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusBadRequest)
						json.NewEncoder(w).Encode(map[string]interface{}{
							"status":  "error",
							"message": fmt.Sprintf("%s 为必填项", field.Label),
						})
						return
					}
				}
			case "checkbox":
				// checkbox 数组检查是否为空数组
				var slice []interface{}
				switch v := val.(type) {
				case []interface{}:
					slice = v
				case []string:
					for _, s := range v {
						slice = append(slice, s)
					}
				default:
					// 其他类型视为单个值
					slice = []interface{}{v}
				}

				if len(slice) == 0 {
					fmt.Printf("    ❌ 验证失败：空数组\n")
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":  "error",
						"message": fmt.Sprintf("%s 为必填项", field.Label),
					})
					return
				}
			}

			fmt.Printf("    ✅ 验证通过\n")
		}
	}

	data["_submitted_at"] = time.Now().Format("2006-01-02 15:04:05")
	data["_ip"] = getClientIP(r)

	// 只保存到数据库，不再保存 JSON 文件
	// if err := h.saveToFile(fi, data); err != nil {
	// 	w.Header().Set("Content-Type", "application/json")
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	json.NewEncoder(w).Encode(map[string]interface{}{
	// 		"status":  "error",
	// 		"message": "保存失败：" + err.Error(),
	// 	})
	// 	return
	// }

	if err := h.saveToDatabase(fi, data); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "数据库保存失败：" + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "数据提交成功",
		"data":    data,
	})
}

func (h *Handler) saveToFile(fi FormInfo, data map[string]interface{}) error {
	dir := fi.DataDirectory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	timestamp := time.Now().UnixNano()
	filename := filepath.Join(dir, fmt.Sprintf("submit_%d.json", timestamp))

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, jsonData, 0644)
}

func (h *Handler) saveToDatabase(fi FormInfo, data map[string]interface{}) error {
	tableName := fi.Model.TableName
	if tableName == "" {
		tableName = "form_" + fi.Name
	}

	if !h.db.TableExists(tableName) {
		// 构建字段列表
		fields := make([]models.FieldInfo, 0, len(fi.Fields))
		for _, f := range fi.Fields {
			fields = append(fields, convertToModelsField(f))
		}

		if err := h.db.CreateTable(tableName, fields); err != nil {
			return err
		}
	}

	return h.db.Insert(tableName, data)
}

// LoginHandler 登录页面
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderTemplate(w, "login.html", map[string]interface{}{
			"Error": "",
		})
		return
	}

	// POST 请求处理登录逻辑
	var loginData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&loginData); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "JSON 解析失败：" + err.Error(),
			})
			return
		}
	} else {
		r.ParseForm()
		loginData.Username = r.FormValue("username")
		loginData.Password = r.FormValue("password")
	}

	// 验证用户名和密码
	hashedPassword, exists := h.adminUsers[loginData.Username]
	if !exists || !verifyPassword(loginData.Password, hashedPassword) {
		if r.Header.Get("Content-Type") == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "用户名或密码错误",
			})
			return
		}
		renderTemplate(w, "login.html", map[string]interface{}{
			"Error": "用户名或密码错误",
		})
		return
	}

	// 创建会话
	sessionID := h.sessionMgr.CreateSession(loginData.Username, 1, "admin")

	// 设置 cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400, // 24 小时
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// 重定向到管理后台
	if r.Header.Get("Content-Type") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "success",
			"message":  "登录成功",
			"redirect": "/admin",
		})
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// LogoutHandler 登出
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		h.sessionMgr.DeleteSession(cookie.Value)
	}

	// 删除 cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// AdminHandler 管理后台首页（需要管理员权限）
func (h *Handler) AdminHandler(w http.ResponseWriter, r *http.Request) {
	// 检查登录状态
	session := h.getCurrentUser(r)
	if session == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// 检查是否为管理员
	if session.Role != "admin" {
		http.Error(w, "无权访问", http.StatusForbidden)
		return
	}

	formList := make([]map[string]interface{}, 0, len(h.formMap))
	for _, fi := range h.formMap {
		// 获取数据条数
		tableName := fi.Model.TableName
		if tableName == "" {
			tableName = "form_" + fi.Name
		}

		count, err := h.db.GetCount(tableName)
		if err != nil {
			count = 0 // 如果表不存在或查询失败，设置为 0
		}

		formList = append(formList, map[string]interface{}{
			"Name":        fi.Name,
			"Title":       fi.Title,
			"Description": fi.Description,
			"FieldCount":  len(fi.Fields),
			"DataCount":   count,          // 添加数据条数
			"FileModTime": fi.FileModTime, // 添加文件修改时间用于排序
		})
	}

	// 按文件修改时间倒序排序（最新的显示在最前面）
	sort.Slice(formList, func(i, j int) bool {
		return formList[i]["FileModTime"].(int64) > formList[j]["FileModTime"].(int64)
	})

	renderTemplate(w, "admin.html", map[string]interface{}{
		"Forms": formList,
		"User":  session,
	})
}

// ExportCSVHandler 导出 CSV 文件
func (h *Handler) ExportCSVHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/export/"), "/")
	if len(parts) < 1 {
		http.NotFound(w, r)
		return
	}
	formName := parts[0]
	fi, exists := h.formMap[formName]
	if !exists {
		http.NotFound(w, r)
		return
	}

	// 确保数据目录存在
	tableName := fi.Model.TableName
	if tableName == "" {
		tableName = "form_" + fi.Name
	}

	// 导出 CSV
	outputPath := fmt.Sprintf("data/%s_%s.csv", fi.Name, time.Now().Format("20060102_150405"))

	// 转换字段类型
	modelsFields := make([]models.FieldInfo, len(fi.Fields))
	for i, f := range fi.Fields {
		modelsFields[i] = convertToModelsField(f)
	}

	if err := h.db.ExportToCSV(tableName, modelsFields, outputPath); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	// 返回下载链接或直接发送文件
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s_%s.csv\"", fi.Name, time.Now().Format("20060102_150405")))

	file, err := os.Open(outputPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "无法读取文件：" + err.Error(),
		})
		return
	}
	defer file.Close()

	io.Copy(w, file)
}

// ViewDataHandler 查看表单数据（JSON 格式）
func (h *Handler) ViewDataHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/data/"), "/")
	if len(parts) < 1 {
		http.NotFound(w, r)
		return
	}
	formName := parts[0]
	fi, exists := h.formMap[formName]
	if !exists {
		http.NotFound(w, r)
		return
	}

	tableName := fi.Model.TableName
	if tableName == "" {
		tableName = "form_" + fi.Name
	}

	// 转换字段类型
	modelsFields := make([]models.FieldInfo, len(fi.Fields))
	for i, f := range fi.Fields {
		modelsFields[i] = convertToModelsField(f)
	}

	data, err := h.db.GetAllData(tableName, modelsFields)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   data,
		"fields": fi.Fields,
	})
}

func getClientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		ips := strings.Split(xf, ",")
		return strings.TrimSpace(ips[0])
	}

	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return xr
	}

	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}
