package main

import (
	"flag"
	"fmt"

	"github.com/golang/glog"
	"github.com/hazaelsan/spandex/expander"
	_ "github.com/hazaelsan/spandex/expander/autokey"
	_ "github.com/hazaelsan/spandex/expander/textexpander"
)

// Command line flags.
var (
	srcExp     = flag.String("source", "", "source expander")
	dstExp     = flag.String("dest", "", "destination expander")
	importName = flag.String("import_name", "", "group name for imported snippets")
)

func main() {
	flag.Parse()
	if *srcExp == "" || *dstExp == "" {
		glog.Exit("-source and -dest must be set")
	}
	if *importName == "" {
		*importName = fmt.Sprintf("Imported from %v", *srcExp)
	}
	src, err := expander.New(*srcExp)
	if err != nil {
		glog.Exit(err)
	}
	if err := src.Load(); err != nil {
		glog.Exit(err)
	}
	dst, err := expander.New(*dstExp)
	if err != nil {
		glog.Exit(err)
	}
	root := &expander.Group{Name: *importName}
	for _, g := range src.Groups() {
		g.Parent = root
	}
	root.MergeAll(src.Groups())
	dst.SetGroup(root)
	if err := dst.Write(); err != nil {
		glog.Exit(err)
	}
}
