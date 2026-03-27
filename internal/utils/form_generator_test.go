package utils

import (
	"testing"
	"go-web/internal/config"
)

func TestGenerateFormHTML(t *testing.T) {
	form := &config.FormConfig{
		Name:        "test_form",
		Title:       "Test Form",
		Description: "Test Description",
		Fields: []*config.FormField{
			{
				Name:     "name",
				Label:    "Name",
				Type:     "text",
				Required: true,
			},
		},
	}

	html, err := GenerateFormHTML(form)
	if err != nil {
		t.Errorf("GenerateFormHTML() error = %v", err)
	}
	if html == "" {
		t.Error("GenerateFormHTML() returned empty string")
	}
}

func TestVersion(t *testing.T) {
	version := Version()
	if version == "" {
		t.Error("Version() returned empty string")
	}
}
