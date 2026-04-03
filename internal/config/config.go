package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   *ServerConfig   `yaml:"server"`
	Database *DatabaseConfig `yaml:"database"`
	Includes []string        `yaml:"includes,omitempty"` // 包含的其他配置文件
	Forms    []*FormConfig   `yaml:"forms"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
	Type string `yaml:"type"`
}

type FormConfig struct {
	Name          string       `yaml:"name"`
	Title         string       `yaml:"title"`
	Description   string       `yaml:"description"`
	Category      string       `yaml:"category,omitempty"`   // 分类，如 hr/marketing/project/survey
	Pinned        bool         `yaml:"pinned,omitempty"`     // 是否置顶
	SortOrder     int          `yaml:"sort_order,omitempty"` // 手工排序，越小越靠前
	Priority      string       `yaml:"priority,omitempty"`   // high/medium/low
	Status        string       `yaml:"status,omitempty"`     // draft/published/archived
	PublishAt     string       `yaml:"publish_at,omitempty"` // 发布时间，支持 RFC3339、2006-01-02 15:04:05、2006-01-02
	ExpireAt      string       `yaml:"expire_at,omitempty"`  // 支持 RFC3339、2006-01-02 15:04:05、2006-01-02
	Fields        []*FormField `yaml:"fields"`
	DataDirectory string       `yaml:"data_directory"`
	Model         *FormModel   `yaml:"model"`
	ConfigSource  string       `yaml:"-"` // 记录配置来源文件
	FileModTime   int64        `yaml:"-"` // 记录配置文件修改时间戳
}

type FormField struct {
	Name        string   `yaml:"name"`
	Label       string   `yaml:"label"`
	Type        string   `yaml:"type"`
	Placeholder string   `yaml:"placeholder"`
	Required    bool     `yaml:"required"`
	Options     []string `yaml:"options"`
	Min         *float64 `yaml:"min"`
	Max         *float64 `yaml:"max"`
	Step        *float64 `yaml:"step"`
	Regex       string   `yaml:"regex"`
}

type FormModel struct {
	TableName string `yaml:"table_name"`
}

// Load 加载主配置文件并合并所有包含的配置
func Load(mainPath string) (*Config, error) {
	// 1. 加载主配置
	mainConfig, err := loadSingleConfig(mainPath)
	if err != nil {
		return nil, err
	}

	// 2. 如果没有 includes，直接返回
	if len(mainConfig.Includes) == 0 {
		return mainConfig, nil
	}

	// 3. 获取主配置所在目录，用于解析相对路径
	baseDir := filepath.Dir(mainPath)

	// 4. 建立已存在表单索引（主配置优先）
	existingForms := make(map[string]struct{}, len(mainConfig.Forms))
	for _, f := range mainConfig.Forms {
		existingForms[f.Name] = struct{}{}
	}

	// 5. 遍历所有 include 配置
	allForms := mainConfig.Forms
	for _, includePattern := range mainConfig.Includes {
		// 支持通配符匹配
		matches, err := filepath.Glob(filepath.Join(baseDir, includePattern))
		if err != nil {
			return nil, fmt.Errorf("匹配配置文件失败：%w", err)
		}

		for _, match := range matches {
			// 跳过主配置本身
			if filepath.Clean(match) == filepath.Clean(mainPath) {
				continue
			}

			// 跳过示例配置，避免覆盖真实配置
			if strings.HasSuffix(filepath.Base(match), ".example.yaml") {
				continue
			}

			includeConfig, err := loadSingleConfig(match)
			if err != nil {
				fmt.Printf("警告：加载配置文件 %s 失败：%v\n", match, err)
				continue
			}

			// 标记来源并做去重（主配置优先，同名忽略）
			addedCount := 0
			for _, form := range includeConfig.Forms {
				if _, exists := existingForms[form.Name]; exists {
					fmt.Printf("⚠️ 跳过重复表单：%s（来源：%s）\n", form.Name, filepath.Base(match))
					continue
				}
				form.ConfigSource = filepath.Base(match)
				allForms = append(allForms, form)
				existingForms[form.Name] = struct{}{}
				addedCount++
			}

			fmt.Printf("✅ 已加载配置文件：%s (新增 %d 个表单)\n", match, addedCount)
		}
	}

	mainConfig.Forms = allForms
	return mainConfig, nil
}

// loadSingleConfig 加载单个配置文件（不处理 includes）
func loadSingleConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 获取文件修改时间
	fileInfo, err := os.Stat(path)
	if err != nil {
		fmt.Printf("警告：无法获取文件 %s 的信息：%v\n", path, err)
	}
	var modTime int64
	if err == nil {
		modTime = fileInfo.ModTime().Unix()
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 设置默认值
	if config.Server == nil {
		config.Server = &ServerConfig{Port: 8080, Host: "localhost"}
	}
	if config.Database == nil {
		config.Database = &DatabaseConfig{Path: "data.db", Type: "sqlite"}
	}

	// 设置默认数据目录并记录文件修改时间
	for _, form := range config.Forms {
		if form.DataDirectory == "" {
			form.DataDirectory = "data/" + form.Name
		}
		if strings.TrimSpace(form.Category) == "" {
			form.Category = "general"
		}
		if strings.TrimSpace(form.Priority) == "" {
			form.Priority = "medium"
		}
		if strings.TrimSpace(form.Status) == "" {
			form.Status = "published"
		}
		form.FileModTime = modTime
	}

	return &config, nil
}
