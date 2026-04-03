package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-web/internal/models"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
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

func (sm *SessionManager) DeleteSessionsByUserID(userID int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, session := range sm.sessions {
		if session.UserID == userID {
			delete(sm.sessions, id)
		}
	}
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
	formMu     sync.RWMutex
	sessionMgr *SessionManager
	configPath string
	reloadFn   func() ([]FormInfo, error)
}

type FormInfo struct {
	Name          string
	Title         string
	Description   string
	Category      string
	Pinned        bool
	SortOrder     int
	Priority      string
	Status        string
	PublishAt     string
	ExpireAt      string // 表单到期时间，支持 RFC3339、2006-01-02 15:04:05、2006-01-02
	DataDirectory string
	Model         struct {
		TableName string
	}
	Fields       []FieldInfo
	FileModTime  int64  // 配置文件修改时间戳
	ConfigSource string // 来源配置文件名，空 = 主配置
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
	Step        *float64
}

func New(db *models.Database, formConfigs []FormInfo, configPath string, reloadFn func() ([]FormInfo, error)) *Handler {
	formMap := make(map[string]FormInfo)
	for _, fi := range formConfigs {
		formMap[fi.Name] = fi
	}

	h := &Handler{
		db:         db,
		formMap:    formMap,
		sessionMgr: NewSessionManager(),
		configPath: configPath,
		reloadFn:   reloadFn,
	}

	if err := h.db.EnsureUserTable(); err != nil {
		panic("初始化用户表失败: " + err.Error())
	}

	if err := h.db.EnsureShareLinkTable(); err != nil {
		panic("初始化分享链接表失败: " + err.Error())
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
		Step:        fi.Step,
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

func parseFormTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}

func normalizeFormStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "draft":
		return "draft"
	case "archived":
		return "archived"
	default:
		return "published"
	}
}

func normalizePriority(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "medium"
	}
}

func statusWeight(status string) int {
	switch normalizeFormStatus(status) {
	case "published":
		return 0
	case "draft":
		return 1
	default:
		return 2
	}
}

func priorityWeight(priority string) int {
	switch normalizePriority(priority) {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}

func (h *Handler) isFormPublished(fi FormInfo) bool {
	return normalizeFormStatus(fi.Status) == "published"
}

func compareFormOrder(a, b FormInfo) bool {
	if a.Pinned != b.Pinned {
		return a.Pinned
	}

	aStatus := statusWeight(a.Status)
	bStatus := statusWeight(b.Status)
	if aStatus != bStatus {
		return aStatus < bStatus
	}

	if a.SortOrder != b.SortOrder {
		return a.SortOrder < b.SortOrder
	}

	aPriority := priorityWeight(a.Priority)
	bPriority := priorityWeight(b.Priority)
	if aPriority != bPriority {
		return aPriority < bPriority
	}

	aPublish, aOk := parseFormTime(a.PublishAt)
	bPublish, bOk := parseFormTime(b.PublishAt)
	if aOk && bOk && !aPublish.Equal(bPublish) {
		return aPublish.After(bPublish)
	}
	if aOk != bOk {
		return aOk
	}

	return strings.ToLower(a.Name) < strings.ToLower(b.Name)
}

func (h *Handler) listForms(includeNonPublished bool) []FormInfo {
	h.formMu.RLock()
	forms := make([]FormInfo, 0, len(h.formMap))
	for _, fi := range h.formMap {
		fi.Status = normalizeFormStatus(fi.Status)
		fi.Priority = normalizePriority(fi.Priority)
		if strings.TrimSpace(fi.Category) == "" {
			fi.Category = "general"
		}

		if !includeNonPublished && !h.isFormPublished(fi) {
			continue
		}
		if !includeNonPublished && h.isFormExpired(fi) {
			continue
		}

		forms = append(forms, fi)
	}
	h.formMu.RUnlock()

	sort.Slice(forms, func(i, j int) bool {
		return compareFormOrder(forms[i], forms[j])
	})

	return forms
}

func normalizeCategory(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return "general"
	}
	return v
}

func normalizeAdminFilterStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "draft":
		return "draft"
	case "archived":
		return "archived"
	case "published":
		return "published"
	default:
		return "all"
	}
}

func toBoolWithDefault(raw string, defaultVal bool) bool {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return defaultVal
	}

	switch raw {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return defaultVal
	}
}

func matchesAdminFilters(fi FormInfo, statusFilter, categoryFilter, keyword string, includeExpired bool, isExpired bool) bool {
	if !includeExpired && isExpired {
		return false
	}

	if statusFilter != "all" && normalizeFormStatus(fi.Status) != statusFilter {
		return false
	}

	if categoryFilter != "all" && normalizeCategory(fi.Category) != categoryFilter {
		return false
	}

	if keyword != "" {
		haystack := strings.ToLower(strings.Join([]string{fi.Name, fi.Title, fi.Description}, " "))
		if !strings.Contains(haystack, keyword) {
			return false
		}
	}

	return true
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

	forms := h.listForms(false)
	formList := make([]map[string]interface{}, 0, len(forms))
	for _, fi := range forms {
		formList = append(formList, map[string]interface{}{
			"Name":        fi.Name,
			"Title":       fi.Title,
			"Description": fi.Description,
			"Category":    fi.Category,
			"Pinned":      fi.Pinned,
			"SortOrder":   fi.SortOrder,
			"Priority":    fi.Priority,
			"Status":      fi.Status,
			"PublishAt":   fi.PublishAt,
			"ExpireAt":    fi.ExpireAt,
		})
	}

	// JSON 响应
	jsonResponse(w, http.StatusOK, formList)
}

func (h *Handler) FormListHandler(w http.ResponseWriter, r *http.Request) {
	forms := h.listForms(false)
	formList := make([]map[string]interface{}, 0, len(forms))
	for _, fi := range forms {
		formList = append(formList, map[string]interface{}{
			"Name":        fi.Name,
			"Title":       fi.Title,
			"Description": fi.Description,
			"Category":    fi.Category,
			"Pinned":      fi.Pinned,
			"SortOrder":   fi.SortOrder,
			"Priority":    fi.Priority,
			"Status":      fi.Status,
			"PublishAt":   fi.PublishAt,
			"ExpireAt":    fi.ExpireAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(formList)
}

func (h *Handler) FormPageHandler(w http.ResponseWriter, r *http.Request) {
	formName := mux.Vars(r)["formName"]
	fi, exists := h.getForm(formName)
	if !exists {
		http.NotFound(w, r)
		return
	}

	if blocked, msg := h.checkFormReadable(fi); blocked {
		jsonResponse(w, http.StatusGone, map[string]string{"error": msg})
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

func (h *Handler) checkFormReadable(fi FormInfo) (bool, string) {
	if !h.isFormPublished(fi) {
		return true, "该表单当前不可填写"
	}
	if h.isFormExpired(fi) {
		return true, "该表单已到期，停止收集"
	}
	return false, ""
}

func normalizePayloadArray(val interface{}) []interface{} {
	switch v := val.(type) {
	case []interface{}:
		return v
	case []string:
		arr := make([]interface{}, 0, len(v))
		for _, item := range v {
			arr = append(arr, item)
		}
		return arr
	default:
		if v == nil {
			return nil
		}
		return []interface{}{v}
	}
}

func (h *Handler) validateRequiredFields(fi FormInfo, data map[string]interface{}) error {
	for _, field := range fi.Fields {
		if !field.Required {
			continue
		}

		val, exists := data[field.Name]
		if !exists || val == nil {
			return fmt.Errorf("%s 为必填项", field.Label)
		}

		switch field.Type {
		case "text", "email", "tel", "url", "password", "textarea", "select", "date", "time", "radio":
			if str, ok := val.(string); ok && strings.TrimSpace(str) == "" {
				return fmt.Errorf("%s 为必填项", field.Label)
			}
		case "number", "range":
			if num, ok := val.(float64); ok && num != num {
				return fmt.Errorf("%s 为必填项", field.Label)
			}
		case "checkbox":
			if len(normalizePayloadArray(val)) == 0 {
				return fmt.Errorf("%s 为必填项", field.Label)
			}
		}
	}

	return nil
}

func (h *Handler) finalizeSubmission(fi FormInfo, data map[string]interface{}, ownerUserID int, ip string) error {
	data["_submitted_at"] = time.Now().Format("2006-01-02 15:04:05")
	data["_ip"] = ip
	data["owner_user_id"] = ownerUserID
	return h.saveToDatabase(fi, data)
}

func generateShareToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func getRequestScheme(r *http.Request) string {
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		parts := strings.Split(proto, ",")
		return strings.TrimSpace(parts[0])
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func firstLocalIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			ipv4 := ip.To4()
			if ipv4 != nil {
				return ipv4.String()
			}
		}
	}

	return ""
}

func resolveShareHost(hostport string) string {
	host := strings.TrimSpace(hostport)
	port := ""

	if strings.Contains(host, ":") {
		if h, p, err := net.SplitHostPort(host); err == nil {
			host = h
			port = p
		}
	}

	host = strings.Trim(host, "[]")
	normalized := strings.ToLower(host)
	if normalized == "" || normalized == "localhost" || normalized == "127.0.0.1" || normalized == "::1" {
		if ip := firstLocalIPv4(); ip != "" {
			host = ip
		}
	}

	if port != "" {
		return net.JoinHostPort(host, port)
	}
	return host
}

func buildShareURL(r *http.Request, token string) string {
	host := resolveShareHost(r.Host)
	return fmt.Sprintf("%s://%s/s/%s", getRequestScheme(r), host, token)
}

func (h *Handler) resolveFormByShareToken(token string) (FormInfo, bool) {
	rec, err := h.db.GetShareLink(token)
	if err != nil || rec == nil {
		return FormInfo{}, false
	}
	fi, exists := h.getForm(rec.FormName)
	if !exists {
		return FormInfo{}, false
	}
	return fi, true
}

func (h *Handler) CreateShareLinkHandler(w http.ResponseWriter, r *http.Request) {
	session := h.getCurrentUser(r)
	if session == nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
		return
	}

	var req struct {
		FormName string `json:"formName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	formName := strings.TrimSpace(req.FormName)
	fi, exists := h.getForm(formName)
	if !exists {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "表单不存在"})
		return
	}

	token, err := generateShareToken()
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "生成链接失败"})
		return
	}

	if err := h.db.CreateShareLink(token, fi.Name, session.UserID); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "保存链接失败"})
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"formName": fi.Name,
		"title":    fi.Title,
		"url":      buildShareURL(r, token),
		"token":    token,
		"expireAt": fi.ExpireAt,
	})
}

func (h *Handler) PublicFormPageHandler(w http.ResponseWriter, r *http.Request) {
	token := mux.Vars(r)["token"]
	fi, ok := h.resolveFormByShareToken(token)
	if !ok {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "分享链接无效"})
		return
	}

	if blocked, msg := h.checkFormReadable(fi); blocked {
		jsonResponse(w, http.StatusGone, map[string]string{"error": msg})
		return
	}

	form := map[string]interface{}{
		"Name":          fi.Name,
		"Title":         fi.Title,
		"Description":   fi.Description,
		"DataDirectory": fi.DataDirectory,
		"Model":         fi.Model,
		"Mode":          "public_share",
	}

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

	jsonResponse(w, http.StatusOK, form)
}

func (h *Handler) PublicSubmitHandler(w http.ResponseWriter, r *http.Request) {
	token := mux.Vars(r)["token"]
	fi, ok := h.resolveFormByShareToken(token)
	if !ok {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "分享链接无效"})
		return
	}

	if blocked, msg := h.checkFormReadable(fi); blocked {
		jsonResponse(w, http.StatusGone, map[string]string{"error": msg})
		return
	}

	data := make(map[string]interface{})
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "JSON 解析失败"})
		return
	}

	if err := h.validateRequiredFields(fi, data); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.finalizeSubmission(fi, data, 0, getClientIP(r)); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "数据库保存失败"})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "数据提交成功",
	})
}

func (h *Handler) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	formName := mux.Vars(r)["formName"]
	fi, exists := h.getForm(formName)
	if !exists {
		http.NotFound(w, r)
		return
	}
	if blocked, msg := h.checkFormReadable(fi); blocked {
		jsonResponse(w, http.StatusGone, map[string]string{"error": msg})
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

	if err := h.validateRequiredFields(fi, data); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

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

	if err := h.finalizeSubmission(fi, data, session.UserID, getClientIP(r)); err != nil {
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

	forms := h.listForms(true)
	q := r.URL.Query()
	statusFilter := normalizeAdminFilterStatus(q.Get("status"))
	categoryFilter := strings.ToLower(strings.TrimSpace(q.Get("category")))
	if categoryFilter == "" {
		categoryFilter = "all"
	}
	keyword := strings.ToLower(strings.TrimSpace(q.Get("keyword")))
	includeExpired := toBoolWithDefault(q.Get("include_expired"), true)

	byStatus := map[string]int{
		"published": 0,
		"draft":     0,
		"archived":  0,
	}
	byCategory := map[string]int{}
	pinnedCount := 0
	expiredCount := 0

	for _, fi := range forms {
		status := normalizeFormStatus(fi.Status)
		category := normalizeCategory(fi.Category)
		byStatus[status]++
		byCategory[category]++
		if fi.Pinned {
			pinnedCount++
		}
		if h.isFormExpired(fi) {
			expiredCount++
		}
	}

	categoryList := make([]string, 0, len(byCategory))
	for category := range byCategory {
		categoryList = append(categoryList, category)
	}
	sort.Strings(categoryList)

	formList := make([]map[string]interface{}, 0, len(forms))
	for _, fi := range forms {
		isExpired := h.isFormExpired(fi)
		if !matchesAdminFilters(fi, statusFilter, categoryFilter, keyword, includeExpired, isExpired) {
			continue
		}

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
			"Category":    normalizeCategory(fi.Category),
			"Pinned":      fi.Pinned,
			"SortOrder":   fi.SortOrder,
			"Priority":    normalizePriority(fi.Priority),
			"Status":      normalizeFormStatus(fi.Status),
			"PublishAt":   fi.PublishAt,
			"ExpireAt":    fi.ExpireAt,
			"IsExpired":   isExpired,
			"FieldCount":  len(fi.Fields),
			"DataCount":   count,
			"FileModTime": fi.FileModTime,
		})
	}

	// JSON 响应
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"forms":               formList,
		"user":                session,
		"availableCategories": categoryList,
		"filters": map[string]interface{}{
			"status":          statusFilter,
			"category":        categoryFilter,
			"keyword":         keyword,
			"include_expired": includeExpired,
		},
		"summary": map[string]interface{}{
			"total":        len(forms),
			"visible":      len(formList),
			"pinnedCount":  pinnedCount,
			"expiredCount": expiredCount,
			"byStatus":     byStatus,
			"byCategory":   byCategory,
		},
	})
}

// ExportCSVHandler 导出 CSV 文件
func (h *Handler) ExportCSVHandler(w http.ResponseWriter, r *http.Request) {
	formName := mux.Vars(r)["formName"]
	fi, exists := h.getForm(formName)
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
	fi, exists := h.getForm(formName)
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

	h.formMu.RLock()
	allForms := make([]FormInfo, 0, len(h.formMap))
	for _, fi := range h.formMap {
		allForms = append(allForms, fi)
	}
	h.formMu.RUnlock()

	for _, fi := range allForms {
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

// CreateUserByAdminHandler 管理员新增用户
func (h *Handler) CreateUserByAdminHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "user"
	}

	if username == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "用户名不能为空"})
		return
	}
	if len(username) < 3 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "用户名至少 3 位"})
		return
	}
	if len(password) < 6 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "密码至少 6 位"})
		return
	}
	if role != "admin" && role != "user" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "角色非法"})
		return
	}

	existing, err := h.db.GetUserByUsername(username)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "查询用户失败"})
		return
	}
	if existing != nil {
		jsonResponse(w, http.StatusConflict, map[string]string{"error": "用户名已存在"})
		return
	}

	uid, err := h.db.CreateUser(username, hashPassword(password), role)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "新增用户失败"})
		return
	}

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"status":   "success",
		"userId":   uid,
		"username": username,
		"role":     role,
	})
}

// DeleteUserByAdminHandler 管理员删除用户
func (h *Handler) DeleteUserByAdminHandler(w http.ResponseWriter, r *http.Request) {
	operator := h.getCurrentUser(r)
	if operator == nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "未登录"})
		return
	}

	idStr := mux.Vars(r)["userId"]
	userID, err := strconv.Atoi(strings.TrimSpace(idStr))
	if err != nil || userID <= 0 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "用户ID非法"})
		return
	}
	if userID == operator.UserID {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "不能删除当前登录用户"})
		return
	}

	targetUser, err := h.db.GetUserByID(userID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "查询用户失败"})
		return
	}
	if targetUser == nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "用户不存在"})
		return
	}
	if strings.EqualFold(strings.TrimSpace(targetUser.Username), "admin") {
		jsonResponse(w, http.StatusForbidden, map[string]string{"error": "admin用户不可删除"})
		return
	}

	if err := h.db.DeleteUser(userID); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "删除用户失败"})
		return
	}

	h.sessionMgr.DeleteSessionsByUserID(userID)

	jsonResponse(w, http.StatusOK, map[string]string{"status": "success"})
}

// ChangePasswordHandler 当前登录用户修改自己的密码
// ImportUsersHandler 管理员批量导入用户
func (h *Handler) ImportUsersHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Users []struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		} `json:"users"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	type FailItem struct {
		Username string `json:"username"`
		Reason   string `json:"reason"`
	}
	failed := make([]FailItem, 0)
	successCount := 0

	for _, u := range req.Users {
		username := strings.TrimSpace(u.Username)
		password := strings.TrimSpace(u.Password)
		role := strings.TrimSpace(u.Role)
		if role == "" {
			role = "user"
		}
		if len(username) < 3 {
			failed = append(failed, FailItem{username, "用户名至少3位"})
			continue
		}
		if len(password) < 6 {
			failed = append(failed, FailItem{username, "密码至少6位"})
			continue
		}
		if role != "admin" && role != "user" {
			failed = append(failed, FailItem{username, "角色须为 user 或 admin"})
			continue
		}
		existing, err := h.db.GetUserByUsername(username)
		if err != nil {
			failed = append(failed, FailItem{username, "查询失败"})
			continue
		}
		if existing != nil {
			failed = append(failed, FailItem{username, "用户名已存在"})
			continue
		}
		if _, err = h.db.CreateUser(username, hashPassword(password), role); err != nil {
			failed = append(failed, FailItem{username, "创建失败"})
			continue
		}
		successCount++
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"total":   len(req.Users),
		"success": successCount,
		"failed":  failed,
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

// getForm 安全获取表单配置（支持并发读取）
func (h *Handler) getForm(name string) (FormInfo, bool) {
	h.formMu.RLock()
	defer h.formMu.RUnlock()
	fi, ok := h.formMap[name]
	return fi, ok
}

// reloadForms 调用外部提供的重载函数，并安全替换 formMap
func (h *Handler) reloadForms() error {
	if h.reloadFn == nil {
		return fmt.Errorf("热重载未配置")
	}
	infos, err := h.reloadFn()
	if err != nil {
		return err
	}
	newMap := make(map[string]FormInfo, len(infos))
	for _, fi := range infos {
		newMap[fi.Name] = fi
	}
	h.formMu.Lock()
	h.formMap = newMap
	h.formMu.Unlock()
	return nil
}

// resolveConfigFilePath 将表单的 ConfigSource 转换为安全的绝对路径
func (h *Handler) resolveConfigFilePath(configSource string) (string, error) {
	if h.configPath == "" {
		return "", fmt.Errorf("配置路径未设置")
	}

	var target string
	if configSource == "" {
		// 来自主配置文件
		target = h.configPath
	} else {
		// 只允许 .yaml/.yml 文件，且不允许目录穿越
		base := filepath.Base(configSource)
		if base == "." || base == ".." || base == "" {
			return "", fmt.Errorf("无效的配置文件名")
		}
		if !strings.HasSuffix(base, ".yaml") && !strings.HasSuffix(base, ".yml") {
			return "", fmt.Errorf("仅支持 .yaml 或 .yml 文件")
		}
		configDir := filepath.Dir(h.configPath)
		target = filepath.Join(configDir, base)
	}

	// 验证路径在 configDir 范围内（防止路径穿越）
	absTarget, err1 := filepath.Abs(target)
	absDir, err2 := filepath.Abs(filepath.Dir(h.configPath))
	if err1 != nil || err2 != nil {
		return "", fmt.Errorf("路径解析失败")
	}
	if !strings.HasPrefix(absTarget, absDir+string(filepath.Separator)) && absTarget != filepath.Clean(h.configPath) {
		return "", fmt.Errorf("路径不合法")
	}

	return target, nil
}

func getFormNameFromNode(node interface{}) string {
	switch v := node.(type) {
	case map[string]interface{}:
		name, _ := v["name"].(string)
		return strings.TrimSpace(name)
	case map[interface{}]interface{}:
		raw, ok := v["name"]
		if !ok {
			return ""
		}
		name, _ := raw.(string)
		return strings.TrimSpace(name)
	default:
		return ""
	}
}

// extractEditableFormsYAML 从完整配置中提取仅可编辑的当前 forms 项
func extractEditableFormsYAML(content []byte, formName string) ([]byte, error) {
	var root map[string]interface{}
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, err
	}

	formsRaw, ok := root["forms"]
	if !ok {
		return nil, fmt.Errorf("未找到 forms 配置")
	}

	forms, ok := formsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("forms 配置格式无效")
	}

	var current interface{}
	for _, form := range forms {
		if getFormNameFromNode(form) == formName {
			current = form
			break
		}
	}
	if current == nil {
		return nil, fmt.Errorf("未找到表单 %s 的配置", formName)
	}

	editable := map[string]interface{}{
		"forms": []interface{}{current},
	}

	return yaml.Marshal(editable)
}

// mergeFormsIntoConfig 将仅包含当前 forms 项的编辑内容合并回完整配置
func mergeFormsIntoConfig(originContent []byte, editedContent []byte, formName string) ([]byte, error) {
	var origin map[string]interface{}
	if err := yaml.Unmarshal(originContent, &origin); err != nil {
		return nil, fmt.Errorf("原配置解析失败: %w", err)
	}

	var edited map[string]interface{}
	if err := yaml.Unmarshal(editedContent, &edited); err != nil {
		return nil, fmt.Errorf("编辑内容解析失败: %w", err)
	}

	for k := range edited {
		if k != "forms" {
			return nil, fmt.Errorf("仅允许编辑 forms 配置")
		}
	}

	forms, ok := edited["forms"]
	if !ok {
		return nil, fmt.Errorf("缺少 forms 配置")
	}
	editedForms, ok := forms.([]interface{})
	if !ok {
		return nil, fmt.Errorf("forms 配置格式无效")
	}
	if len(editedForms) != 1 {
		return nil, fmt.Errorf("仅允许编辑当前表单对应项")
	}
	if getFormNameFromNode(editedForms[0]) != formName {
		return nil, fmt.Errorf("仅允许编辑表单 %s", formName)
	}

	originFormsRaw, ok := origin["forms"]
	if !ok {
		return nil, fmt.Errorf("原配置缺少 forms 配置")
	}
	originForms, ok := originFormsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("原配置 forms 格式无效")
	}

	replaced := false
	for i, form := range originForms {
		if getFormNameFromNode(form) == formName {
			originForms[i] = editedForms[0]
			replaced = true
			break
		}
	}
	if !replaced {
		return nil, fmt.Errorf("原配置中未找到表单 %s", formName)
	}

	origin["forms"] = originForms
	return yaml.Marshal(origin)
}

// GetFormConfigHandler 返回表单所在的 YAML 文件原始内容
func (h *Handler) GetFormConfigHandler(w http.ResponseWriter, r *http.Request) {
	formName := mux.Vars(r)["formName"]
	fi, ok := h.getForm(formName)
	if !ok {
		// 尝试直接读主配置（表单可能刚创建）
		fi = FormInfo{Name: formName, ConfigSource: ""}
	}

	filePath, err := h.resolveConfigFilePath(fi.ConfigSource)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "读取配置文件失败: " + err.Error()})
		return
	}

	editableContent, err := extractEditableFormsYAML(content, formName)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "配置解析失败: " + err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"formName": formName,
		"source":   filepath.Base(filePath),
		"content":  string(editableContent),
	})
}

// SaveFormConfigHandler 验证并保存 YAML 内容，然后热重载表单
func (h *Handler) SaveFormConfigHandler(w http.ResponseWriter, r *http.Request) {
	formName := mux.Vars(r)["formName"]
	fi, ok := h.getForm(formName)
	if !ok {
		fi = FormInfo{Name: formName, ConfigSource: ""}
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
		return
	}

	// 验证 YAML 语法
	var probe interface{}
	if err := yaml.Unmarshal([]byte(req.Content), &probe); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "YAML 格式错误: " + err.Error()})
		return
	}

	filePath, err := h.resolveConfigFilePath(fi.ConfigSource)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	originContent, err := os.ReadFile(filePath)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "读取原配置失败: " + err.Error()})
		return
	}

	mergedContent, err := mergeFormsIntoConfig(originContent, []byte(req.Content), formName)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// 安全写入：先写临时文件再改名
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, mergedContent, 0644); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "写入手失败: " + err.Error()})
		return
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "保存配置文件失败: " + err.Error()})
		return
	}

	// 热重载
	if err := h.reloadForms(); err != nil {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"status":  "saved_reload_failed",
			"message": "配置已保存，但重载失败: " + err.Error(),
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "配置已保存并重载",
	})
}
