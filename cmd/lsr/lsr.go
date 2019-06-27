// Program lsr lists directory recursively.
package main

import (
	"flag"
	"fmt"
	"os"
	pathutil "path/filepath"
)

var prunixtime = flag.Bool("t", false, "print mode, modification time, and size")
var printdirs = flag.Bool("d", false, "print directories also")

var exitCode = 0

func report(err error) {
	fmt.Fprintln(os.Stderr, err)
	exitCode = 1
}

func pr(path string, d os.FileInfo) {
	if *prunixtime {
		fmt.Printf("%s %o %d %d\n", path,
			d.Mode(), d.ModTime().Unix(), d.Size())
	} else {
		fmt.Printf("%s\n", path)
	}
}

func walker(path string, info os.FileInfo, err error) error {
	if err != nil {
		report(err)
		if info.IsDir() {
			return pathutil.SkipDir
		}
		return nil
	}
	if info.IsDir() && *printdirs {
		pr(path+"/", info)
	}
	if !info.IsDir() {
		pr(path, info)
	}
	return nil
}

func main() {
	flag.Parse()

	for i := 0; i < flag.NArg(); i++ {
		pathutil.Walk(flag.Arg(i), walker)
	}
	if flag.NArg() == 0 {
		pathutil.Walk(".", walker)
	}
	os.Exit(exitCode)
}
