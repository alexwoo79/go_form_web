package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-web/internal/models"
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

	"github.com/gorilla/mux"
)

// JSON 响应工具函数
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
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
}

type FormInfo struct {
	Name          string
	Title         string
	Description   string
	ExpireAt      string // 表单到期时间，支持 RFC3339、2006-01-02 15:04:05、2006-01-02
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
	}

	if err := h.db.EnsureUserTable(); err != nil {
		panic("初始化用户表失败: " + err.Error())
	}

	adminUser, err := h.db.GetUserByUsername("admin")
	if err != nil {
		panic("检查默认管理员失败: " + err.Error())
	}
	if adminUser == nil {
		defaultPassword := hashPassword("admin123")
		if _, err := h.db.CreateUser("admin", defaultPassword, "admin"); err != nil {
			panic("创建默认管理员失败: " + err.Error())
		}
	}

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

// RequireAdmin 检查用户是否已登录且为管理员（中间件），未授权返回 401 JSON
func (h *Handler) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		session := h.sessionMgr.GetSession(cookie.Value)
		if session == nil || session.Role != "admin" {
			jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "权限不足"})
			return
		}

		next(w, r)
	}
}

func (h *Handler) RequireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		session := h.sessionMgr.GetSession(cookie.Value)
		if session == nil {
			jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "会话已失效"})
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

func parseExpireAt(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid expire_at format: %s", raw)
}

func (h *Handler) isFormExpired(fi FormInfo) bool {
	if strings.TrimSpace(fi.ExpireAt) == "" {
		return false
	}

	expireAt, err := parseExpireAt(fi.ExpireAt)
	if err != nil {
		// 配置格式错误时不阻塞服务，仅忽略过期判断。
		fmt.Printf("⚠️ 表单 %s 的 expire_at 配置无效：%v\n", fi.Name, err)
		return false
	}

	return time.Now().After(expireAt)
}

// MeHandler 返回当前登录用户信息，未登录返回 401
func (h *Handler) MeHandler(w http.ResponseWriter, r *http.Request) {
	session := h.getCurrentUser(r)
	if session == nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":       session.UserID,
		"username": session.Username,
		"role":     session.Role,
	})
}

func (h *Handler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	formList := make([]map[string]interface{}, 0, len(h.formMap))
	for _, fi := range h.formMap {
		if h.isFormExpired(fi) {
			continue
		}
		formList = append(formList, map[string]interface{}{
			"Name":        fi.Name,
			"Title":       fi.Title,
			"Description": fi.Description,
			"ExpireAt":    fi.ExpireAt,
		})
	}

	// JSON 响应
	jsonResponse(w, http.StatusOK, formList)
}

func (h *Handler) FormListHandler(w http.ResponseWriter, r *http.Request) {
	formList := make([]map[string]interface{}, 0, len(h.formMap))
	for _, fi := range h.formMap {
		if h.isFormExpired(fi) {
			continue
		}
		formList = append(formList, map[string]interface{}{
			"Name":        fi.Name,
			"Title":       fi.Title,
			"Description": fi.Description,
			"ExpireAt":    fi.ExpireAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(formList)
}

func (h *Handler) FormPageHandler(w http.ResponseWriter, r *http.Request) {
	formName := mux.Vars(r)["formName"]
	fi, exists := h.formMap[formName]
	if !exists {
		http.NotFound(w, r)
		return
	}
	if h.isFormExpired(fi) {
		jsonResponse(w, http.StatusGone, map[string]string{"error": "该表单已到期，停止收集"})
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

	// JSON 响应
	jsonResponse(w, http.StatusOK, form)
}

func (h *Handler) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	formName := mux.Vars(r)["formName"]
	fi, exists := h.formMap[formName]
	if !exists {
		http.NotFound(w, r)
		return
	}
	if h.isFormExpired(fi) {
		jsonResponse(w, http.StatusGone, map[string]string{"error": "该表单已到期，停止收集"})
		return
	}

	session := h.getCurrentUser(r)
	if session == nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "请先登录后再提交"})
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
			case "number", "range":
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
			case "number", "range":
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
	data["owner_user_id"] = session.UserID

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

// RegisterHandler 用户注册
func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) < 3 || len(req.Username) > 32 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "用户名长度需为 3-32"})
		return
	}
	if len(req.Password) < 6 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "密码至少 6 位"})
		return
	}

	user, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "查询用户失败"})
		return
	}
	if user != nil {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "用户名已存在"})
		return
	}

	passwordHash := hashPassword(req.Password)
	uid, err := h.db.CreateUser(req.Username, passwordHash, "user")
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "创建用户失败"})
		return
	}

	sessionID := h.sessionMgr.CreateSession(req.Username, int(uid), "user")
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"status": "success",
		"user": map[string]interface{}{
			"id":       int(uid),
			"username": req.Username,
			"role":     "user",
		},
	})
}

// LoginHandler 用户登录
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// 仅接受 POST + JSON
	var loginData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&loginData); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	user, err := h.db.GetUserByUsername(strings.TrimSpace(loginData.Username))
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "查询用户失败"})
		return
	}

	if user == nil || !verifyPassword(loginData.Password, user.PasswordHash) {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
		return
	}

	// 创建会话并设置 cookie
	sessionID := h.sessionMgr.CreateSession(user.Username, user.ID, user.Role)
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"user": map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
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

	jsonResponse(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// AdminHandler 管理后台首页（需要管理员权限）
func (h *Handler) AdminHandler(w http.ResponseWriter, r *http.Request) {
	session := h.getCurrentUser(r)

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

	// JSON 响应
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"forms": formList,
		"user":  session,
	})
}

// ExportCSVHandler 导出 CSV 文件
func (h *Handler) ExportCSVHandler(w http.ResponseWriter, r *http.Request) {
	formName := mux.Vars(r)["formName"]
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
	formName := mux.Vars(r)["formName"]
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

// MySubmissionsHandler 查询当前用户的提交记录
func (h *Handler) MySubmissionsHandler(w http.ResponseWriter, r *http.Request) {
	session := h.getCurrentUser(r)
	if session == nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
		return
	}

	items := make([]map[string]interface{}, 0)

	for _, fi := range h.formMap {
		tableName := fi.Model.TableName
		if tableName == "" {
			tableName = "form_" + fi.Name
		}
		if !h.db.TableExists(tableName) {
			continue
		}

		rows, err := h.db.Query(tableName, "owner_user_id = ?", session.UserID)
		if err != nil {
			continue
		}

		for _, row := range rows {
			record := map[string]interface{}{
				"formName":    fi.Name,
				"formTitle":   fi.Title,
				"submittedAt": row["_submitted_at"],
				"ip":          row["_ip"],
				"fields":      fi.Fields,
				"data":        map[string]interface{}{},
			}

			payload := make(map[string]interface{})
			for _, f := range fi.Fields {
				if val, ok := row[f.Name]; ok {
					payload[f.Name] = val
				}
			}
			record["data"] = payload
			items = append(items, record)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		left, _ := items[i]["submittedAt"].(string)
		right, _ := items[j]["submittedAt"].(string)
		return left > right
	})

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"items":  items,
	})
}

// UpdateUserRoleHandler 管理员更新用户角色
func (h *Handler) UpdateUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID int    `json:"userId"`
		Role   string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}
	if req.Role != "admin" && req.Role != "user" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "角色非法"})
		return
	}
	if req.UserID <= 0 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "用户ID非法"})
		return
	}

	targetUser, err := h.db.GetUserByID(req.UserID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "查询用户失败"})
		return
	}
	if targetUser == nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "用户不存在"})
		return
	}
	if strings.EqualFold(strings.TrimSpace(targetUser.Username), "admin") {
		jsonResponse(w, http.StatusForbidden, map[string]string{"error": "admin用户角色不可修改"})
		return
	}

	if err := h.db.UpdateUserRole(req.UserID, req.Role); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "更新角色失败"})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "success"})
}

// ListUsersHandler 管理员查看用户列表
func (h *Handler) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.ListUsers()
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "查询用户失败"})
		return
	}

	result := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		result = append(result, map[string]interface{}{
			"id":        u.ID,
			"username":  u.Username,
			"role":      u.Role,
			"createdAt": u.CreatedAt,
		})
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"items":  result,
	})
}

// ChangePasswordHandler 当前登录用户修改自己的密码
func (h *Handler) ChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	session := h.getCurrentUser(r)
	if session == nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
		return
	}

	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	if len(req.NewPassword) < 6 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "新密码至少 6 位"})
		return
	}

	u, err := h.db.GetUserByID(session.UserID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "查询用户失败"})
		return
	}
	if u == nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "用户不存在"})
		return
	}

	if !verifyPassword(req.OldPassword, u.PasswordHash) {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "原密码错误"})
		return
	}

	if err := h.db.UpdateUserPassword(session.UserID, hashPassword(req.NewPassword)); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "更新密码失败"})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "success"})
}

// AdminUpdateUserPasswordHandler 管理员修改指定用户密码
func (h *Handler) AdminUpdateUserPasswordHandler(w http.ResponseWriter, r *http.Request) {
	operator := h.getCurrentUser(r)
	if operator == nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
		return
	}

	var req struct {
		UserID      int    `json:"userId"`
		NewPassword string `json:"newPassword"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	if req.UserID <= 0 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "用户ID非法"})
		return
	}
	if len(req.NewPassword) < 6 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "新密码至少 6 位"})
		return
	}

	u, err := h.db.GetUserByID(req.UserID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "查询用户失败"})
		return
	}
	if u == nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "用户不存在"})
		return
	}

	if strings.EqualFold(strings.TrimSpace(u.Username), "admin") && !strings.EqualFold(strings.TrimSpace(operator.Username), "admin") {
		jsonResponse(w, http.StatusForbidden, map[string]string{"error": "admin密码仅允许admin账户本人修改"})
		return
	}

	if err := h.db.UpdateUserPassword(req.UserID, hashPassword(req.NewPassword)); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "更新密码失败"})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "success"})
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
