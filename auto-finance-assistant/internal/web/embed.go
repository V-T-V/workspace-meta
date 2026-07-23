// Package web 嵌入 Vue 构建产物并提供 SPA 静态服务。
// 对应原计划 4.4 + 21.1：前端编译后由 Go 服务直接托管。
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var distFS embed.FS

// StaticHandler 返回托管前端静态资源的 http.Handler。
// 带简易 SPA fallback：找不到文件时回退到 index.html。
func StaticHandler() http.Handler {
	sub, _ := fs.Sub(distFS, "dist")
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /api/ 路径不应落到 SPA，返回 404 JSON
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"接口不存在"}}`))
			return
		}
		clean := strings.TrimPrefix(r.URL.Path, "/")
		// 静态资源直接命中
		if clean != "" && resourceExists(sub, clean) {
			fileServer.ServeHTTP(w, r)
			return
		}
		// 其余路径回退 index.html（SPA 客户端路由）
		serveIndex(w, r, sub)
	})
}

// resourceExists 检查 embed 中是否存在某文件。
func resourceExists(fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return !stat.IsDir()
}

// serveIndex 返回 index.html。
func serveIndex(w http.ResponseWriter, _ *http.Request, fsys fs.FS) {
	data, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		http.Error(w, "前端资源缺失", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
