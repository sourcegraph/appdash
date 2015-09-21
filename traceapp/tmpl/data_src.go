// +build dev

package tmpl

import (
	"net/http"
	"os"
	pathpkg "path"

	"github.com/shurcooL/go/vfs/httpfs/filter"
)

var Assets = filter.NewIgnore(
	http.Dir(rootDir),
	func(fi os.FileInfo, _ string) bool {
		return !fi.IsDir() && pathpkg.Ext(fi.Name()) == ".go"
	},
)
