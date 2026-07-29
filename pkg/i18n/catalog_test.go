package i18n

import (
	"testing"
	"testing/fstest"
)

func TestCatalogTranslate(t *testing.T) {
	catalog, err := NewCatalog()
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	if got := catalog.Translate(LocaleZhCN, "common.success", nil); got != "操作成功" {
		t.Fatalf("Chinese success = %q", got)
	}
	if got := catalog.Translate(LocaleEn, "common.success", nil); got != "Success" {
		t.Fatalf("English success = %q", got)
	}
}

func TestCatalogInterpolation(t *testing.T) {
	localeFS := fstest.MapFS{
		"locales/zh-CN.json": {Data: []byte(`{"error.internal_server":"服务器错误","greeting":"你好，{{.name}}"}`)},
		"locales/en.json":    {Data: []byte(`{"error.internal_server":"Server error","greeting":"Hello, {{.name}}"}`)},
	}
	catalog, err := NewCatalogFromFS(localeFS)
	if err != nil {
		t.Fatalf("NewCatalogFromFS() error = %v", err)
	}
	if got := catalog.Translate(LocaleEn, "greeting", Params{"name": "FileDock"}); got != "Hello, FileDock" {
		t.Fatalf("translated greeting = %q", got)
	}
}

func TestCatalogRejectsInvalidResources(t *testing.T) {
	tests := []struct {
		name string
		fs   fstest.MapFS
	}{
		{
			name: "invalid json",
			fs: fstest.MapFS{
				"locales/zh-CN.json": {Data: []byte(`{`)},
				"locales/en.json":    {Data: []byte(`{}`)},
			},
		},
		{
			name: "missing key",
			fs: fstest.MapFS{
				"locales/zh-CN.json": {Data: []byte(`{"error.internal_server":"服务器错误","success":"成功"}`)},
				"locales/en.json":    {Data: []byte(`{"error.internal_server":"Server error"}`)},
			},
		},
		{
			name: "empty value",
			fs: fstest.MapFS{
				"locales/zh-CN.json": {Data: []byte(`{"error.internal_server":"服务器错误","success":""}`)},
				"locales/en.json":    {Data: []byte(`{"error.internal_server":"Server error","success":"Success"}`)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewCatalogFromFS(tt.fs); err == nil {
				t.Fatal("NewCatalogFromFS() error = nil")
			}
		})
	}
}

func TestCatalogUsesSafeFallbackAndReportsMissingKey(t *testing.T) {
	reported := false
	catalog, err := NewCatalog(WithMissingKeyHandler(func(_ Locale, _ string, err error) {
		reported = err != nil
	}))
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	if got := catalog.Translate(LocaleEn, "missing.key", nil); got != "服务器繁忙，请稍后重试" {
		t.Fatalf("safe fallback = %q", got)
	}
	if !reported {
		t.Fatal("missing key was not reported")
	}
}
