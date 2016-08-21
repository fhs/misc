package main

import (
	"bufio"
	"errors"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode"

	"9fans.net/go/acme"
	"9fans.net/go/plan9"
	"9fans.net/go/plumb"
)

func ReadAddr(win *acme.Win) (q0, q1 int, err error) {
	// addr is reset to 0,0 once opened, so make sure it's already open
	if _, _, err := win.ReadAddr(); err != nil {
		return 0, 0, err
	}
	if err := win.Ctl("addr=dot"); err != nil {
		return 0, 0, err
	}
	return win.ReadAddr()
}

func IsIdentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func IdentAtAddr(b []rune, q0, q1 int) (string, error) {
	if q0 == q1 {
		// expand characters before cursor
		q0--
		for q0 >= 0 && q0 < len(b) && IsIdentRune(b[q0]) {
			q0--
		}
		q0++

		// expand characters after cursor
		for q1 >= 0 && q1 < len(b) && IsIdentRune(b[q1]) {
			q1++
		}
	}
	if q1 > q0 {
		return string(b[q0:q1]), nil
	}
	return "", errors.New("not found")
}

func GetIdent() (string, error) {
	winid := os.Getenv("winid")
	if len(winid) == 0 {
		return "", errors.New("$winid not set")
	}
	id, err := strconv.Atoi(winid)
	if err != nil {
		return "", err
	}
	win, err := acme.Open(id, nil)
	if err != nil {
		return "", err
	}
	if err := win.Ctl("addr=dot"); err != nil {
		return "", err
	}
	q0, q1, err := ReadAddr(win)
	if err != nil {
		return "", err
	}
	body, err := win.ReadAll("body")
	if err != nil {
		return "", err
	}
	return IdentAtAddr([]rune(string(body)), q0, q1)
}

func ParseTag(line string) (ident, filename, addr string) {
	i := strings.Index(line, `;"`)
	if i >= 0 {
		line = line[:i]
	}
	e := strings.SplitN(strings.TrimSpace(line), "\t", 3)
	if len(e) != 3 {
		log.Fatalf("invalid tag entry %q", line)
	}
	return e[0], e[1], e[2]
}

func FindTag(tagfile, ident string) (filename, addr string, err error) {
	f, err := os.Open(tagfile)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", "", err
		}
		if len(line) > 0 && line[0] == '!' {
			continue
		}
		if len(line) > 0 {
			id, filename, addr := ParseTag(line)
			if id == ident {
				return filename, addr, nil
			}
		}
		if err == io.EOF {
			break
		}
	}
	return "", "", nil
}

func PlumbTag(filename, addr string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	f, err := plumb.Open("send", plan9.OWRITE)
	if err != nil {
		return err
	}
	defer f.Close()

	m := &plumb.Message{
		Src:  "acme",
		Dst:  "edit",
		Dir:  wd,
		Type: "text",
		Attr: &plumb.Attribute{Name: "addr", Value: addr},
		Data: []byte(filename),
	}
	return m.Send(f)
}

func main() {
	var ident string
	if len(os.Args) >= 2 {
		ident = os.Args[1]
	} else {
		var err error
		ident, err = GetIdent()
		if err != nil {
			log.Fatalf("failed to get identifier: %v\n", err)
		}
	}

	filename, addr, err := FindTag("tags", ident)
	if err != nil {
		log.Fatalf("failed to parse tags file: %v\n", err)
	}

	if err := PlumbTag(filename, addr); err != nil {
		log.Fatalf("failed to plumb: %v\n", err)
	}
}
