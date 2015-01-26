package tmpl

import (
	"go/build"
	"log"
	"os"
	"path/filepath"
)

// TemplateDir is the directory containing the html/template template files.
var templateDir = filepath.Join(defaultBase("sourcegraph.com/sourcegraph/apptrace/traceapp"), "tmpl")

func defaultBase(path string) string {
	p, err := build.Default.Import(path, "", build.FindOnly)
	if err != nil {
		log.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	p.Dir, err = filepath.Rel(cwd, p.Dir)
	if err != nil {
		log.Fatal(err)
	}
	return p.Dir
}
