package main

import (
	"collector"
	"github.com/forensicanalysis/fslib/filesystem/systemfs"
)

func main() {
	sourceFS, _ := systemfs.New()
	collector.Collect("test", sourceFS)
}
