// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Adapted from https://code.google.com/p/rsc/source/browse/cmd/acmego/main.go

// Acmepy watches acme for .py files being written.
// Each time a .py file is written, acmepy checks whether the
// (4-space) indentations need to be fixed. If so, it makes the
// changes in the window body but does not write the file.
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"code.google.com/p/goplan9/plan9/acme"
)

var LineRef = regexp.MustCompile(`File "([a-zA-Z¡-￿0-9_\-./]+)", line ([0-9]+)`)

func main() {
	l, err := acme.Log()
	if err != nil {
		log.Fatal(err)
	}

	for {
		event, err := l.Read()
		if err != nil {
			log.Fatal(err)
		}
		if event.Name != "" && event.Op == "put" && strings.HasSuffix(event.Name, ".py") {
			reformat(event.ID, event.Name)
		}
	}
}

func reformat(id int, name string) {
	w, err := acme.Open(id, nil)
	if err != nil {
		log.Print(err)
		return
	}
	defer w.CloseFiles()

	old, err := ioutil.ReadFile(name)
	if err != nil {
		//log.Print(err)
		return
	}
	cmd := exec.Command("/usr/lib/python2.7/Tools/scripts/reindent.py")
	cmd.Stdin = bytes.NewReader(old)
	new, err := cmd.CombinedOutput()
	if err != nil {
		// Probably a syntax error, use the compiler for better message.
		cmd := exec.Command("python", "-m", "py_compile", name)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", LineRef.ReplaceAll(out, []byte("File $1:$2")))
			return
		}
		return
	}

	if bytes.Equal(old, new) {
		return
	}

	f, err := ioutil.TempFile("", "acmepy")
	if err != nil {
		log.Print(err)
		return
	}
	if _, err := f.Write(new); err != nil {
		log.Print(err)
		return
	}
	tmp := f.Name()
	f.Close()
	defer os.Remove(tmp)

	diff, _ := exec.Command("9", "diff", name, tmp).CombinedOutput()

	w.Write("ctl", []byte("mark"))
	w.Write("ctl", []byte("nomark"))
	diffLines := strings.Split(string(diff), "\n")
	for i := len(diffLines) - 1; i >= 0; i-- {
		line := diffLines[i]
		if line == "" {
			continue
		}
		if line[0] == '<' || line[0] == '-' || line[0] == '>' {
			continue
		}
		j := 0
		for j < len(line) && line[j] != 'a' && line[j] != 'c' && line[j] != 'd' {
			j++
		}
		if j >= len(line) {
			log.Printf("cannot parse diff line: %q", line)
			break
		}
		oldStart, oldEnd := parseSpan(line[:j])
		newStart, newEnd := parseSpan(line[j+1:])
		if oldStart == 0 || newStart == 0 {
			continue
		}
		switch line[j] {
		case 'a':
			err := w.Addr("%d+#0", oldStart)
			if err != nil {
				log.Print(err)
				break
			}
			w.Write("data", findLines(new, newStart, newEnd))
		case 'c':
			err := w.Addr("%d,%d", oldStart, oldEnd)
			if err != nil {
				log.Print(err)
				break
			}
			w.Write("data", findLines(new, newStart, newEnd))
		case 'd':
			err := w.Addr("%d,%d", oldStart, oldEnd)
			if err != nil {
				log.Print(err)
				break
			}
			w.Write("data", nil)
		}
	}
}

func parseSpan(text string) (start, end int) {
	i := strings.Index(text, ",")
	if i < 0 {
		n, err := strconv.Atoi(text)
		if err != nil {
			log.Printf("cannot parse span %q", text)
			return 0, 0
		}
		return n, n
	}
	start, err1 := strconv.Atoi(text[:i])
	end, err2 := strconv.Atoi(text[i+1:])
	if err1 != nil || err2 != nil {
		log.Printf("cannot parse span %q", text)
		return 0, 0
	}
	return start, end
}

func findLines(text []byte, start, end int) []byte {
	i := 0

	start--
	for ; i < len(text) && start > 0; i++ {
		if text[i] == '\n' {
			start--
			end--
		}
	}
	startByte := i
	for ; i < len(text) && end > 0; i++ {
		if text[i] == '\n' {
			end--
		}
	}
	endByte := i
	return text[startByte:endByte]
}
