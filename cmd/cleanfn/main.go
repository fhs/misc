// A command to clean up file names. Characters such as space and punctuations
// are removed so that the file name works well with the shell. It outputs
// mv(1) commands to rename files, which can be piped to the sh(1) to do the
// actual renaming.
package main

import (
	"fmt"
	"os"
	"strings"
)

func isSeparator(r rune) bool {
	switch r {
	case '-', '_', '.':
		return true
	}
	return false
}

func combineSeparators(s string) string {
	s = strings.TrimLeftFunc(s, isSeparator)
	s = strings.TrimRightFunc(s, isSeparator)

	notSeparator := func(r rune) bool {
		return !isSeparator(r)
	}
	result := ""
	for s != "" {
		t := strings.TrimLeftFunc(s, notSeparator)
		result += s[:len(s)-len(t)]
		s = t
		t = strings.TrimLeftFunc(s, isSeparator)
		sep := s[:len(s)-len(t)]
		if len(sep) > 1 {
			result += "."
		} else {
			result += sep
		}
		s = t
	}
	return result
}

func mapRune(r rune) rune {
	switch r {
	case '`', '\'', '"':
		return -1
	case '^', '#', '*', '[', ']', '=', '|', '?',
		'$', '{', '}', '(', ')', '<', '>', '&',
		';', '\\', '!', '~', '+', ',', ' ':
		return '_'
	}
	return r
}

func replaceChars(s string) string {
	s = strings.Replace(s, "&", "_and_", -1)
	return strings.Map(mapRune, s)
}

func clean(s string) string {
	return combineSeparators(replaceChars(s))
}

func exists(filename string) bool {
	if _, err := os.Stat(filename); err != nil {
		return os.IsExist(err)
	}
	return true
}

func main() {
	for _, src := range os.Args[1:] {
		if strings.Contains(src, "/") {
			fmt.Printf("# %q contains path separator\n", src)
			continue
		}
		if dst := clean(src); dst != src {
			flags := ""
			if src[0] == '-' {
				flags = "--"
			}
			if exists(dst) {
				fmt.Printf("# mv %s %q %q	# file exists\n", flags, src, dst)
			} else {
				fmt.Printf("mv %s %q %q\n", flags, src, dst)
			}
		}
	}
}
