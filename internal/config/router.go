package config

import (
	"go-web/internal/handler"
	"net/http"
	"github.com/gorilla/mux"
)

func NewRouter(h *handler.Handler) *mux.Router {
	r := mux.NewRouter()

	// 公开路由
	r.HandleFunc("/", h.IndexHandler).Methods("GET")
	r.HandleFunc("/login", h.LoginHandler).Methods("GET", "POST")
	r.HandleFunc("/logout", h.LogoutHandler).Methods("GET", "POST")
	r.HandleFunc("/forms", h.FormListHandler).Methods("GET")
	r.HandleFunc("/forms/{formName}", h.FormPageHandler).Methods("GET")
	
	// API 路由（需要登录）
	r.HandleFunc("/api/submit/{formName}", h.SubmitHandler).Methods("POST")
	r.HandleFunc("/api/export/{formName}", h.RequireAdmin(h.ExportCSVHandler)).Methods("GET")
	r.HandleFunc("/api/data/{formName}", h.RequireAdmin(h.ViewDataHandler)).Methods("GET")

	// 管理后台（需要管理员权限）
	r.HandleFunc("/admin", h.RequireAdmin(h.AdminHandler)).Methods("GET")

	// 静态文件
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("ui/static"))))
	r.PathPrefix("/gen/").Handler(http.StripPrefix("/gen/", http.FileServer(http.Dir("generated"))))

	return r
}
