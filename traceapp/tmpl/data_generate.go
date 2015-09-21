// +build generate

package main

import (
	"log"

	"github.com/shurcooL/vfsgen"
	"sourcegraph.com/sourcegraph/appdash/traceapp/tmpl"
)

func main() {
	err := vfsgen.Generate(tmpl.Assets, vfsgen.Options{
		Filename:     "data.go",
		PackageName:  "tmpl",
		BuildTags:    "!dev",
		VariableName: "Assets",
	})
	if err != nil {
		log.Fatalln(err)
	}
}
