// Package webui embute o viewer web compilado (web/dist → internal/webui/dist).
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist retorna a árvore de arquivos do viewer compilado.
func Dist() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}
