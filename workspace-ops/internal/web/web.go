// Package web 嵌入 Vue 前端构建产物（dist/）+ fallback HTML。
// 开发时 dist 可能只有占位（未 build），HasDist() 返回 false 时用 FallbackHTML。
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
)

//go:embed dist/*
var distFS embed.FS

//go:embed fallback.html
var fallbackHTML []byte

// HasDist 报告 embed 的 dist 目录是否有真实构建产物（.gitkeep 占位除外）。
func HasDist() bool {
	entries, err := distFS.ReadDir("dist")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() != ".gitkeep" {
			return true
		}
	}
	return false
}

// StaticHandler 返回前端静态资源的 http.Handler（serve 子命令用）。
// 有真实 dist 时从 dist 提供；否则返回 fallback.html。
//
// SPA fallback 策略：先尝试打开请求的文件，存在则直接 serve；
// 不存在则把路径重写成 /（index.html），交给前端路由处理。
// 这样 Vue 的 /assets/app.js 等真实静态资源能正常返回，
// 而 /projects/123 这类前端路由也回退到 index.html。
func StaticHandler() http.Handler {
	if !HasDist() {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(fallbackHTML)
		})
	}
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "前端资源不可用", http.StatusInternalServerError)
		})
	}
	fileSrv := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 根路径直接 serve index.html。
		if r.URL.Path == "/" {
			fileSrv.ServeHTTP(w, r)
			return
		}
		// 尝试打开请求的文件，不存在则 fallback 到 index.html（SPA 路由）。
		// distFS 是 embed.FS，用 distFS.Open 检查文件存在性。
		fullpath := filepath.Join("dist", filepath.Clean(r.URL.Path))
		if _, err := distFS.Open(fullpath); err != nil {
			r2 := new(http.Request)
			*r2 = *r
			r2.URL.Path = "/"
			fileSrv.ServeHTTP(w, r2)
			return
		}
		fileSrv.ServeHTTP(w, r)
	})
}
