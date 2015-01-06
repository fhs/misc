// This command read a MODIS granule name from standard input (one per line)
// and outputs the URL of where a MODIS product corresponding to the same
// date/time can be downloaded.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/jlaffaye/ftp"
)

type Product struct {
	FilenamePat, PrefixTempl, URLTempl string
	//FilenameRE *regexp.Regexp
	//PrefixT, URLT *template.Template
}

var Products = make(map[string]*Product, 10)

func init() {
	MODISProductNames := []string{
		"MOD03", "MOD021KM", "MOD02HKM", "MOD02QKM",
		"MYD03", "MYD021KM", "MYD02HKM", "MYD02QKM",
	}
	MODISTimeTempl := `.A{{printf "%04d" .Year}}{{printf "%03d" .YDay}}.{{printf "%02d" .Hour}}{{printf "%02d" .Min}}.`
	MODISTimePat := `\.A(?P<year>\d\d\d\d)(?P<yday>\d\d\d)\.(?P<hour>\d\d)(?P<min>\d\d)\.`
	for _, name := range MODISProductNames {
		Products[name] = &Product{
			FilenamePat: "^" + name + MODISTimePat,
			PrefixTempl: name + MODISTimeTempl,
			URLTempl:    `ftp://ladsweb.nascom.nasa.gov:ftp/allData/5/` + name + "/" + `{{printf "%04d" .Year}}/{{printf "%03d" .YDay}}/`,
		}
	}
	Products["MYD10_L2"] = &Product{
		FilenamePat: `^MYD10_L2` + MODISTimePat,
		PrefixTempl: "MYD10_L2" + MODISTimeTempl,
		URLTempl:    `ftp://n5eil01u.ecs.nsidc.org:ftp/SAN/MOSA/MYD10_L2.005/{{printf "%04d" .Year}}.{{printf "%02d" .Mon}}.{{printf "%02d" .MDay}}/`,
	}
	Products["MOD10_L2"] = &Product{
		FilenamePat: `^MOD10_L2` + MODISTimePat,
		PrefixTempl: "MOD10_L2" + MODISTimeTempl,
		URLTempl:    `ftp://n5eil01u.ecs.nsidc.org:ftp/SAN/MOST/MOD10_L2.005/{{printf "%04d" .Year}}.{{printf "%02d" .Mon}}.{{printf "%02d" .MDay}}/`,
	}
}

type Time struct {
	Year, MDay, YDay, Hour, Min int
	Mon                         time.Month
}

func (t Time) Time() time.Time {
	return time.Date(t.Year, t.Mon, t.MDay, t.Hour, t.Min, 0, 0, time.UTC)
}

func IsLeap(year int) bool {
	return year%400 == 0 || (year%4 == 0 && year%100 != 0)
}

func YDay2MDay(year, yday int) (time.Month, int, error) {
	if year < 0 || yday < 0 {
		return -1, -1, errors.New("invalid year or yday")
	}
	mdays := []int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	if IsLeap(year) {
		mdays[time.February]++
	}
	for m := time.January; m <= time.December; m++ {
		if yday <= mdays[m] {
			return m, yday, nil
		}
		yday -= mdays[m]
	}
	return -1, -1, errors.New("yday too large")
}

// Caches used to reduce network I/O
var FTPConnections = make(map[string]*ftp.ServerConn, 5)
var URLEntries = make(map[string][]string, 10)

func FTPList(u *url.URL) (entries []string, err error) {
	if u.Scheme != "ftp" {
		err = fmt.Errorf("unsupported URL scheme %s", u.Scheme)
		return
	}
	c, ok := FTPConnections[u.Host]
	if !ok {
		c, err = ftp.Connect(u.Host)
		if err != nil {
			return
		}
		err = c.Login("anonymous", "ftp@example.com")
		if err != nil {
			return
		}
		FTPConnections[u.Host] = c
	}
	// Some ftp servers return rooted paths for NameList
	// if we don't ChangeDir (e.g. ftp://n5eil01u.ecs.nsidc.org)
	err = c.ChangeDir(u.Path)
	if err != nil {
		return
	}
	entries, err = c.NameList(".")
	return
}

func SearchURL(rawurl string, prefix string) (newurl string) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return
	}
	entries, ok := URLEntries[rawurl]
	if !ok {
		entries, err = FTPList(u)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ftp error:", err)
			return
		}
		URLEntries[rawurl] = entries
	}
	for _, ent := range entries {
		if strings.HasPrefix(ent, prefix) {
			newurl = "ftp://" + path.Join(u.Host, u.Path, ent)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "granule with prefix %s not found\n", prefix)
	return
}

func (prod *Product) GetTime(filename string) (t *Time, err error) {
	re := regexp.MustCompile(prod.FilenamePat)
	matches := re.FindStringSubmatch(filename)
	if matches == nil {
		return nil, errors.New("no date matched")
	}
	names := re.SubexpNames()

	subexp := make(map[string]int)
	for i, m := range matches[1:] {
		n, err := strconv.Atoi(m)
		if err != nil {
			return nil, err
		}
		subexp[names[i+1]] = n
	}
	t = &Time{
		Year: subexp["year"],
		Hour: subexp["hour"],
		Min:  subexp["min"],
	}
	if yday, ok := subexp["yday"]; ok {
		t.YDay = yday
		t.Mon, t.MDay, err = YDay2MDay(t.Year, t.YDay)
		if err != nil {
			return
		}
	} else {
		t.MDay = subexp["mday"]
		t.Mon = time.Month(subexp["mon"])
		t.YDay = t.Time().YearDay()
	}
	return
}

func (prod *Product) GetURLAt(t *Time) (string, error) {
	var buf bytes.Buffer

	templ := template.Must(template.New("url").Parse(prod.URLTempl))
	if err := templ.Execute(&buf, t); err != nil {
		return "", err
	}
	rawurl := buf.String()
	buf.Reset()

	templ = template.Must(template.New("prefix").Parse(prod.PrefixTempl))
	if err := templ.Execute(&buf, t); err != nil {
		return "", err
	}
	prefix := buf.String()

	return SearchURL(rawurl, prefix), nil
}

func (prod *Product) GetURL(oldname string) (newurl string, err error) {
	var t *Time

	for _, prod := range Products {
		t, err = prod.GetTime(oldname)
		if err == nil {
			break
		}
	}
	if t == nil {
		return "", fmt.Errorf("unknown product file %s", oldname)
	}
	return prod.GetURLAt(t)
}

func cleanup() {
	for _, c := range FTPConnections {
		_ = c.Quit()
	}
}

func fatalln(v ...interface{}) {
	cleanup()
	log.Fatalln(v...)
}

func productNames() []string {
	names := []string{}
	for name := range Products {
		names = append(names, name)
	}
	return names
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s <productname>\n", os.Args[0])
		names := sort.StringSlice(productNames())
		names.Sort()
		fmt.Fprintf(os.Stderr, "\nAvailable products: %s\n\n",
			strings.Join(names, ", "))
		flag.PrintDefaults()
		os.Exit(1)
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
	}
	prodname := flag.Arg(0)
	product, ok := Products[prodname]
	if !ok {
		fatalln("bad product name", prodname)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		oldname := scanner.Text()
		newurl, err := product.GetURL(oldname)
		if err != nil {
			fatalln("failed to get URL:", err)
		}
		if newurl != "" {
			//fmt.Println(oldname, "=>", newurl)
			fmt.Println(newurl)
		}
	}
	if err := scanner.Err(); err != nil {
		fatalln("reading standard input:", err)
	}
	cleanup()
}
