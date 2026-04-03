package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"go-web/internal/config"
	"go-web/internal/handler"
	"go-web/internal/models"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func ensureRuntimeDirs(dbPath string) error {
	if dbPath == "" {
		return fmt.Errorf("ensureRuntimeDirs: dbPath must not be empty")
	}

	absDBPath, err := filepath.Abs(dbPath)
	if err != nil {
		return fmt.Errorf("ensureRuntimeDirs: resolve absolute path for %q: %w", dbPath, err)
	}

	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("ensureRuntimeDirs: create data directory: %w", err)
	}

	dbDir := filepath.Dir(absDBPath)
	if dbDir != "." && dbDir != "" {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return fmt.Errorf("ensureRuntimeDirs: create database directory %q: %w", dbDir, err)
		}
	}

	return nil
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
			if ipv4 := ip.To4(); ipv4 != nil {
				return ipv4.String()
			}
		}
	}

	return ""
}

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	port := flag.String("port", "8080", "服务端口")
	tlsCert := flag.String("tls-cert", "", "TLS 证书文件路径（与 --tls-key 同时提供时启用 HTTPS）")
	tlsKey := flag.String("tls-key", "", "TLS 私钥文件路径（与 --tls-cert 同时提供时启用 HTTPS）")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	if err := ensureRuntimeDirs(cfg.Database.Path); err != nil {
		log.Fatalf("创建运行目录失败: %v", err)
	}

	// 初始化数据库
	db, err := models.NewDatabase(cfg.Database.Path, cfg.Database.Type)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
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

		tableName := fc.Model.TableName
		if tableName == "" {
			tableName = "form_" + fc.Name
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
			DataDirectory: "",
			Model:         struct{ TableName string }{TableName: tableName},
			Fields:        fields,
			FileModTime:   fc.FileModTime,
			ConfigSource:  fc.ConfigSource,
		})

		// 动态创建或更新数据库表结构
		// 转换为 models.FieldInfo
		modelsFields := make([]models.FieldInfo, len(fields))
		for i, f := range fields {
			modelsFields[i] = models.FieldInfo{
				Name:        f.Name,
				Label:       f.Label,
				Type:        f.Type,
				Placeholder: f.Placeholder,
				Required:    f.Required,
				Options:     f.Options,
				Min:         f.Min,
				Max:         f.Max,
				Step:        f.Step,
			}
		}

		if db.TableExists(tableName) {
			// 表已存在，尝试更新结构（添加新列）
			log.Printf("检测到表 %s 已存在，检查是否需要更新结构...", tableName)
			// 简化处理：直接尝试添加新列
			if err := db.UpdateTableSchema(tableName, nil, modelsFields); err != nil {
				log.Printf("警告：更新表结构失败：%v", err)
			}
		} else {
			// 创建新表
			if err := db.CreateTable(tableName, modelsFields); err != nil {
				log.Fatalf("创建表 %s 失败：%v", tableName, err)
			}
			log.Printf("已创建数据表：%s", tableName)
		}
	}

	// 热重载函数：重新读取配置并返回最新的 FormInfo 列表
	reloadFn := func() ([]handler.FormInfo, error) {
		newCfg, err := config.Load(*configPath)
		if err != nil {
			return nil, err
		}
		var infos []handler.FormInfo
		for _, fc := range newCfg.Forms {
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
			tn := fc.Model.TableName
			if tn == "" {
				tn = "form_" + fc.Name
			}
			infos = append(infos, handler.FormInfo{
				Name:         fc.Name,
				Title:        fc.Title,
				Description:  fc.Description,
				Category:     fc.Category,
				Pinned:       fc.Pinned,
				SortOrder:    fc.SortOrder,
				Priority:     fc.Priority,
				Status:       fc.Status,
				PublishAt:    fc.PublishAt,
				ExpireAt:     fc.ExpireAt,
				Model:        struct{ TableName string }{TableName: tn},
				Fields:       fields,
				FileModTime:  fc.FileModTime,
				ConfigSource: fc.ConfigSource,
			})
		}
		return infos, nil
	}

	// 初始化处理器
	h := handler.New(db, formInfos, *configPath, reloadFn)

	// 创建路由（使用 gorilla/mux）
	r := config.NewRouter(h)

	// 健康检查端点
	r.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}).Methods(http.MethodGet)

	listenAddr := fmt.Sprintf("0.0.0.0:%s", *port)
	log.Printf("服务器启动成功，监听地址：%s", listenAddr)
	if ip := firstLocalIPv4(); ip != "" {
		log.Printf("访问：http://%s:%s", ip, *port)
		log.Printf("管理后台：http://%s:%s/admin", ip, *port)
	} else {
		log.Printf("访问：http://localhost:%s", *port)
		log.Printf("管理后台：http://localhost:%s/admin", *port)
	}

	// 启动日志
	for _, fi := range formInfos {
		log.Printf("表单已加载：%s (%s)", fi.Title, fi.Name)
	}

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 在后台 goroutine 启动服务器
	serverErr := make(chan error, 1)
	go func() {
		if *tlsCert != "" && *tlsKey != "" {
			log.Printf("TLS 已启用，使用证书：%s", *tlsCert)
			serverErr <- srv.ListenAndServeTLS(*tlsCert, *tlsKey)
		} else {
			serverErr <- srv.ListenAndServe()
		}
	}()

	// 捕获 SIGINT / SIGTERM，实现优雅关机
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务器启动失败：%v", err)
		}
	case <-ctx.Done():
		log.Printf("收到退出信号，开始优雅关机...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("服务器关闭时发生错误：%v", err)
	}
	if err := db.Close(); err != nil {
		log.Printf("数据库关闭时发生错误：%v", err)
	}
	log.Printf("服务器已关闭")
}
