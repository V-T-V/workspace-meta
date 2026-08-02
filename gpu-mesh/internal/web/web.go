// Package web 内嵌 GPU Mesh 控制台前端（embed.FS，单二进制部署，无构建步骤）。
package web

import "embed"

//go:embed index.html
var FS embed.FS
