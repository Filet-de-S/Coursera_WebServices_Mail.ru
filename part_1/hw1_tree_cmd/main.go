package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
)

type mainVars struct {
	out             io.Writer
	printFiles      bool
	offsetPrintable int
	offsetTab       int
}

type fileDir struct {
	fileName string
	bytes    string
	isDir    bool
}

func printTree(dirs *[]fileDir, v *mainVars) error {
	var absPath, printPath string
	l := len(*dirs) - 1
	if l == 0 {
		return nil // only one path, no files to print
	}
	if (*dirs)[0].fileName != "/" {
		absPath = (*dirs)[0].fileName
	}

	off := "│	"
	for n := 1; n <= l; n++ {
		if n == l {
			printPath = "└───" + (*dirs)[n].fileName + (*dirs)[n].bytes
		} else {
			printPath = "├───" + (*dirs)[n].fileName + (*dirs)[n].bytes
		}
		offset := ""
		for i := 0; i < v.offsetPrintable; i++ {
			offset += off
		}
		for i := 0; i < v.offsetTab; i++ {
			offset += "\t"
		}
		if _, err := fmt.Fprint(v.out, offset); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(v.out, printPath); err != nil {
			return err
		}
		if (*dirs)[n].isDir == true {
			offPrint, offTabs := v.offsetPrintable, v.offsetTab
			if n == l {
				offTabs++
			} else {
				offPrint++
			}
			vars := &mainVars{
				out:             v.out,
				printFiles:      v.printFiles,
				offsetPrintable: offPrint,
				offsetTab:       offTabs,
			}
			if err := dirTreeRun(absPath+"/"+(*dirs)[n].fileName, vars); err != nil {
				return err
			}
		}
	}
	return nil
}

func getSortedDir(files []os.FileInfo, printFiles bool, abs string) *[]fileDir {
	if len(files) == 0 {
		return nil
	}
	var myDir = make([]fileDir, 1)

	for f := range files {
		fileName := files[f].Name()
		isDir := files[f].IsDir()
		if isDir == false && printFiles == true {
			var value string
			if files[f].Size() == 0 {
				value = " (empty)"
			} else {
				value = " (" + strconv.FormatInt(files[f].Size(), 10) + "b" + ")"
			}
			myDir = append(myDir, fileDir{fileName: fileName, bytes: value})
		} else if isDir == true {
			myDir = append(myDir, fileDir{fileName: fileName, isDir: true})
		}
	}

	sort.SliceStable(myDir, func(i, j int) bool {
		return myDir[i].fileName < myDir[j].fileName
	})
	myDir[0] = fileDir{fileName: abs}

	return &myDir
}

func dirTreeRun(path string, vars *mainVars) error {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("can't open dir: %w", err)
	}

	files, err := file.Readdir(0)
	if err != nil {
		fmt.Errorf("error during reading dir: %w", err)
	}
	if len(files) == 0 {
		return nil
	}

	currentDir := getSortedDir(files, vars.printFiles, path)
	return printTree(currentDir, vars)
}

func dirTree(out io.Writer, path string, printFiles bool) error {
	vars := &mainVars{out: out, printFiles: printFiles}
	return dirTreeRun(path, vars)
}

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}
