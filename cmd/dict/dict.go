// Program dict is a Dictionary Server Protocol (RFC 2229) client.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/net/dict"
)

var (
	showdicts = flag.Bool("D", false, "show list of available dictionaries")
	dictname  = flag.String("d", "*", "the dictionary to use")
	addr      = flag.String("a", "dict.org:dict", "dict server address")
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] word\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	conn, err := dict.Dial("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if *showdicts {
		dicts, err := conn.Dicts()
		if err != nil {
			log.Fatal(err)
		}
		for _, d := range dicts {
			fmt.Printf("%s: %s\n", d.Name, d.Desc)
		}
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	defn, err := conn.Define(*dictname, flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range defn {
		fmt.Printf("From %s [%s]:\n\n", d.Dict.Desc, d.Dict.Name)
		fmt.Printf("%s\n", d.Text)
	}
}
