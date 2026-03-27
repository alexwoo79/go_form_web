package main

import (
	"flag"
	"fmt"
	"go-web/internal/config"
	"go-web/internal/utils"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	outputDir := flag.String("output", "generated", "输出目录")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建输出目录
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("创建输出目录失败: %v", err)
	}

	// 生成表单
	for _, form := range cfg.Forms {
		log.Printf("正在生成表单: %s", form.Name)

		// 生成表单 HTML
		html, err := utils.GenerateFormHTML(form)
		if err != nil {
			log.Printf("生成 HTML 失败: %v", err)
			continue
		}

		// 保存 HTML 文件
		filename := fmt.Sprintf("%s/%s.html", *outputDir, form.Name)
		if err := ioutil.WriteFile(filename, []byte(html), 0644); err != nil {
			log.Printf("保存 HTML 文件失败: %v", err)
			continue
		}

		log.Printf("表单 HTML 生成成功: %s", filename)
	}

	// 生成索引页面
	indexHTML := generateIndexPage(cfg.Forms)
	indexFile := fmt.Sprintf("%s/index.html", *outputDir)
	if err := ioutil.WriteFile(indexFile, []byte(indexHTML), 0644); err != nil {
		log.Fatalf("生成索引页面失败: %v", err)
	}

	log.Printf("所有表单生成完成！基本路径: http://localhost:8080/")
}

func generateIndexPage(forms []*config.FormConfig) string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
    <title>表单中心</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; }
        h1 { text-align: center; color: #333; }
        .form-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 20px; }
        .form-card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .form-card h3 { margin-top: 0; color: #2c3e50; }
        .form-card p { color: #666; }
        .form-link { display: inline-block; margin-top: 15px; padding: 10px 20px; background: #3498db; color: white; text-decoration: none; border-radius: 5px; }
        .form-link:hover { background: #2980b9; }
    </style>
</head>
<body>
    <h1>📝 表单中心</h1>
    <div class="form-list">`)

	for _, form := range forms {
		sb.WriteString(fmt.Sprintf(`
        <div class="form-card">
            <h3>%s</h3>
            <p>%s</p>
            <a href="/forms/%s" class="form-link">填写表单</a>
        </div>`, form.Title, form.Description, form.Name))
	}

	sb.WriteString(`
    </div>
</body>
</html>`)

	return sb.String()
}
