package collector

import (
	"fmt"
	"github.com/forensicanalysis/fslib"
	"path"
)

type BitCollector struct {
	sourceFS fslib.FS
	tempDir  string
}

type PbFile struct {
	ID         string
	Artifact   string
	Size       float64
	Name       string
	Ctime      string
	Mtime      string
	Atime      string
	ExportPath string
}

func Collect(temp string, source fslib.FS) {
	collector := BitCollector{sourceFS: source, tempDir: temp}
	collector.CollectFile("C:\\$MFT", source)
	return
}

func (collector *BitCollector) CollectFile(name string, source fslib.FS) {
	file := collector.CreateFile("$MFT", name)

	if file == nil {
		fmt.Println(file)
	}
	return
}

func (collector *BitCollector) CreateFile(definitionName string, srcPath string) *PbFile {
	file := PbFile{Artifact: definitionName, Name: path.Base(srcPath)}

	srcInfo, _ := collector.sourceFS.Stat(srcPath)
	file.Size = float64(srcInfo.Size())
	// attr := srcInfo.Sys()

	return &file
}
