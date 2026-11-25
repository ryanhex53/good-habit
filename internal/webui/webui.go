// Package webui 嵌入前端构建产物（web/ 经 Vite 构建到本目录 dist/）。
// 空仓库时 dist 仅含 .keep 占位，`all:` 前缀保证空目录也能编译。
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist 是前端静态资源根（index.html + assets/*）——已剥掉 embed 的 dist/ 前缀。
var Dist fs.FS = mustSub(distFS, "dist")

func mustSub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic("webui: " + err.Error())
	}
	return sub
}
