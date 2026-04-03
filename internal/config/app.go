package config

import (
	"go-web/internal/handler"
	"go-web/internal/models"
	"go-web/ui"
	"io/fs"
	"net/http"
)

type App struct {
	config  *Config
	handler *handler.Handler
	db      *models.Database
}

func NewApp(configPath string) (*App, error) {
	cfg, err := Load(configPath)
	if err != nil {
		return nil, err
	}

	db, err := models.NewDatabase(cfg.Database.Path, cfg.Database.Type)
	if err != nil {
		return nil, err
	}

	// 转换表单配置
	formInfos := make([]handler.FormInfo, 0, len(cfg.Forms))
	for _, fc := range cfg.Forms {
		fields := make([]handler.FieldInfo, 0, len(fc.Fields))
		for _, f := range fc.Fields {
			fields = append(fields, handler.FieldInfo{
				Name:        f.Name,
				Label:       f.Label,
				Type:        f.Type,
				Placeholder: f.Placeholder,
				Required:    f.Required,
				Options:     f.Options,
				Min:         f.Min,
				Max:         f.Max,
				Step:        f.Step,
			})
		}

		formInfos = append(formInfos, handler.FormInfo{
			Name:          fc.Name,
			Title:         fc.Title,
			Description:   fc.Description,
			Category:      fc.Category,
			Pinned:        fc.Pinned,
			SortOrder:     fc.SortOrder,
			Priority:      fc.Priority,
			Status:        fc.Status,
			PublishAt:     fc.PublishAt,
			ExpireAt:      fc.ExpireAt,
			DataDirectory: fc.DataDirectory,
			Model:         struct{ TableName string }{TableName: fc.Model.TableName},
			Fields:        fields,
			FileModTime:   fc.FileModTime,
			ConfigSource:  fc.ConfigSource,
		})
	}

	h := handler.New(db, formInfos, "", nil)

	return &App{
		config:  cfg,
		handler: h,
		db:      db,
	}, nil
}

func (a *App) Close() {
	if a.db != nil {
		a.db.Close()
	}
}

func (a *App) Router() http.Handler {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(ui.Static, "static")
	if err != nil {
		panic("加载内嵌静态资源失败: " + err.Error())
	}

	mux.HandleFunc("/", a.handler.IndexHandler)
	mux.HandleFunc("/forms", a.handler.FormListHandler)
	mux.HandleFunc("/forms/", a.handler.FormPageHandler)
	mux.HandleFunc("/api/submit/", a.handler.SubmitHandler)

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.Handle("/gen/", http.StripPrefix("/gen/", http.FileServer(http.Dir("generated"))))

	return mux
}
