// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Modified from https://github.com/9fans/go/blob/master/acme/editinacme/main.go

// Goplay is a Go playground for acme(1).
//
// Goplay uses the plumber to ask acme to open a temporary file,
// and runs the file everytime Put is executed for that file. Once
// the file's acme window is deleted, it removes the temporary file,
// and exits.
//
// Run with acmego (http://godoc.org/code.google.com/p/rsc/cmd/acmego)
// for import path rewriting.
package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"

	"9fans.net/go/acme"
)

var HelloProg = `package main

import "fmt"

func main() {
	fmt.Println("Hello, 世界")
}
`

func run(filename string) {
	cmd := exec.Command("go", "run", filename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("%s\n", err)
	}
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("goplay: ")

	dir, err := ioutil.TempDir("", "goplay")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	file := path.Join(dir, "a.go")
	if err := ioutil.WriteFile(file, []byte(HelloProg), 0600); err != nil {
		log.Fatal(err)
	}

	r, err := acme.Log()
	if err != nil {
		log.Fatal(err)
	}

	out, err := exec.Command("plumb", "-d", "edit", file).CombinedOutput()
	if err != nil {
		log.Fatalf("executing plumb: %v\n%s", err, out)
	}

	for {
		ev, err := r.Read()
		if err != nil {
			log.Fatalf("reading acme log: %v", err)
		}
		if ev.Op == "del" && ev.Name == file {
			break
		}
		if ev.Op == "put" && ev.Name == file {
			run(file)
		}
	}
}
