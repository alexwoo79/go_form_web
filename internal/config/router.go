package config

import (
	"go-web/internal/handler"
	"go-web/ui"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gorilla/mux"
)

// spaHandler 处理 Vue SPA：优先返回静态资源，不存在时回退到 index.html。
type spaHandler struct {
	assets    fs.FS
	fileSrv   http.Handler
	indexHTML []byte
}

func newEmbeddedSPAHandler() (*spaHandler, error) {
	frontendFS, err := fs.Sub(ui.Frontend, "frontend")
	if err != nil {
		return nil, err
	}

	indexHTML, err := fs.ReadFile(frontendFS, "index.html")
	if err != nil {
		return nil, err
	}

	return &spaHandler{
		assets:    frontendFS,
		fileSrv:   http.FileServer(http.FS(frontendFS)),
		indexHTML: indexHTML,
	}, nil
}

func newDiskSPAHandler(distDir string) *spaHandler {
	return &spaHandler{
		assets:    os.DirFS(distDir),
		fileSrv:   http.FileServer(http.Dir(distDir)),
		indexHTML: nil,
	}
}

func (s *spaHandler) ensureIndexLoaded() bool {
	if s.indexHTML != nil {
		return true
	}
	indexHTML, err := fs.ReadFile(s.assets, "index.html")
	if err != nil {
		return false
	}
	s.indexHTML = indexHTML
	return true
}

func (s *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if reqPath == "" || reqPath == "." {
		reqPath = "index.html"
	}

	if reqPath != "index.html" {
		if f, err := s.assets.Open(reqPath); err == nil {
			if info, statErr := f.Stat(); statErr == nil && !info.IsDir() {
				_ = f.Close()
				s.fileSrv.ServeHTTP(w, r)
				return
			}
			_ = f.Close()
		}
	}

	if !s.ensureIndexLoaded() {
		http.Error(w, "frontend assets not found, run ./build.sh", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(s.indexHTML)
}

func NewRouter(h *handler.Handler) *mux.Router {
	r := mux.NewRouter()
	spa := &spaHandler{}

	if embedded, err := newEmbeddedSPAHandler(); err == nil {
		spa = embedded
	} else if _, statErr := os.Stat("./vue-form/dist/index.html"); statErr == nil {
		spa = newDiskSPAHandler("./vue-form/dist")
	} else {
		// Both embedded and disk frontend are unavailable; keep handler to return clear error.
		spa = &spaHandler{assets: os.DirFS("."), fileSrv: http.NotFoundHandler()}
	}

	// API 路由 - 返回 JSON
	r.HandleFunc("/api/register", h.RegisterHandler).Methods("POST")
	r.HandleFunc("/api/login", h.LoginHandler).Methods("POST")
	r.HandleFunc("/api/logout", h.LogoutHandler).Methods("POST")
	r.HandleFunc("/api/me", h.MeHandler).Methods("GET")
	r.HandleFunc("/api/user/password", h.RequireLogin(h.ChangePasswordHandler)).Methods("POST")
	r.HandleFunc("/api/forms", h.RequireLogin(h.FormListHandler)).Methods("GET")
	r.HandleFunc("/api/forms/{formName}", h.RequireLogin(h.FormPageHandler)).Methods("GET")
	r.HandleFunc("/api/submit/{formName}", h.RequireLogin(h.SubmitHandler)).Methods("POST")
	r.HandleFunc("/api/my/submissions", h.RequireLogin(h.MySubmissionsHandler)).Methods("GET")
	r.HandleFunc("/api/export/{formName}", h.RequireAdmin(h.ExportCSVHandler)).Methods("GET")
	r.HandleFunc("/api/data/{formName}", h.RequireAdmin(h.ViewDataHandler)).Methods("GET")
	r.HandleFunc("/api/admin", h.RequireAdmin(h.AdminHandler)).Methods("GET")
	r.HandleFunc("/api/admin/share-links", h.RequireAdmin(h.CreateShareLinkHandler)).Methods("POST")
	r.HandleFunc("/api/admin/form-config/{formName}", h.RequireAdmin(h.GetFormConfigHandler)).Methods("GET")
	r.HandleFunc("/api/admin/form-config/{formName}", h.RequireAdmin(h.SaveFormConfigHandler)).Methods("PUT")
	r.HandleFunc("/api/admin/users", h.RequireAdmin(h.ListUsersHandler)).Methods("GET")
	r.HandleFunc("/api/admin/users", h.RequireAdmin(h.CreateUserByAdminHandler)).Methods("POST")
	r.HandleFunc("/api/admin/users/import", h.RequireAdmin(h.ImportUsersHandler)).Methods("POST")
	r.HandleFunc("/api/admin/users/{userId}", h.RequireAdmin(h.DeleteUserByAdminHandler)).Methods("DELETE")
	r.HandleFunc("/api/admin/user-role", h.RequireAdmin(h.UpdateUserRoleHandler)).Methods("POST")
	r.HandleFunc("/api/admin/user-password", h.RequireAdmin(h.AdminUpdateUserPasswordHandler)).Methods("POST")
	r.HandleFunc("/api/public/forms/{token}", h.PublicFormPageHandler).Methods("GET")
	r.HandleFunc("/api/public/submit/{token}", h.PublicSubmitHandler).Methods("POST")

	// 所有其他路由由 Vue SPA 处理
	r.PathPrefix("/").Handler(spa)

	return r
}
