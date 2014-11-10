package main

// http://cefs.steve-meier.de/errata.latest.xml

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type XMLOpt struct {
	Packages    []string `xml:"packages"`
	Release     int      `xml:"release,attr"`
	Product     string   `xml:"product,attr"`
	References  string   `xml:"references,attr"`
	Type        string   `xml:"type,attr"`
	Topic       string   `xml:"topic,attr"`
	OsRelease   []int    `xml:"os_release"`
	OsArch      []string `xml:"os_arch"`
	Severity    string   `xml:"severity,attr"`
	Solution    string   `xml:"solution,attr"`
	Notes       string   `xml:"notes,attr"`
	Synopsis    string   `xml:"synopsis,attr"`
	Description string   `xml:"description,attr"`
}

type XMLOpts struct {
	Opt []XMLOpt `xml:",any"`
}

var lastModified time.Time

func ShouldRefreshErrata() bool {
	resp, err := http.Head("http://cefs.steve-meier.de/errata.latest.xml")
	if err != nil {
		return true
	}

	// Example: Fri, 31 Oct 2014 09:40:46 GMT
	const longForm = "Fri, 2 Jan 2006 3:04:05 MST"
	_lastModified, err := time.Parse(longForm, resp.Header.Get("Last-Modified"))
	if err != nil {
		panic(err)
	}

	// Dont compare Time objects, make sure they're something comparible first.
	if fmt.Sprintf("%s", _lastModified) != fmt.Sprintf("%s", lastModified) {
		lastModified = _lastModified
		return true
	} else {
		return false
	}
}

func GetSecurityErrata() []byte {
	file, err := ioutil.ReadFile("errata.latest.xml")
	if err != nil {
		panic(err)
	}
	return file
}

func ParseSecurityErrata() []XMLOpt {
	v := XMLOpts{}
	errata := GetSecurityErrata()
	err := xml.Unmarshal(errata, &v)
	if err != nil {
		fmt.Printf("error: %v", err)
		return nil
	}

	security := []XMLOpt{}
	for _, pkg := range v.Opt {
		if pkg.Type == "Security Advisory" {
			for _, ver := range pkg.OsRelease {
				if versionLUT[ver] == nil {
					versionLUT[ver] = packageLUT{}
				}
				if versionLUT[ver][pkg.Release] == nil {
					versionLUT[ver][pkg.Release] = []XMLOpt{}
				}
				versionLUT[ver][pkg.Release] = append(versionLUT[ver][pkg.Release], pkg)
			}
		}
	}
	return security
}

func CheckForUpdates() {
	if ShouldRefreshErrata() {
		ParseSecurityErrata()
	}
}

type packageLUT map[int][]XMLOpt

var versionLUT map[int]packageLUT = map[int]packageLUT{}

func AppendIfMissing(slice []XMLOpt, x XMLOpt) []XMLOpt {
	for _, ele := range slice {
		// HEREEEEEEEEEEEEE
		// DO SOMETHING WITH .hash on struct to compare something
		if ele.Hash() == x.Hash() {
			return slice
		}
	}
	return append(slice, x)
}

func (p *XMLOpt) Hash() string {
	return fmt.Sprintf("%s%s%s", p.OsRelease, strings.Join(p.Packages, "."), p.Release)
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/"):]
	pathArr := strings.Split(path, "/")
	_, ver, pkg, rel := pathArr[1], pathArr[2], pathArr[3], pathArr[4]
	version, _ := strconv.ParseInt(ver, 10, 0)
	release, _ := strconv.ParseInt(rel, 10, 0)

	xpkgs := versionLUT[int(version)][int(release)]
	respPkgs := []XMLOpt{}
	for _, xpkg := range xpkgs {
		for _, vpkg := range xpkg.Packages {
			if strings.Contains(vpkg, pkg) {
				respPkgs = AppendIfMissing(respPkgs, xpkg)
			}
		}
	}
	resp, err := json.Marshal(respPkgs)

	if err != nil {
		fmt.Println(err)
		resp, _ = json.Marshal(struct{ Message string }{fmt.Sprintf("Invalid json: %s", err)})
	}

	fmt.Fprintln(w, string(resp))
}

func main() {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for t := range ticker.C {
			fmt.Println("[ ] Checking for updates....", t)
			CheckForUpdates()
			fmt.Println("[x] Checking for updates....", t)
		}
	}()

	http.HandleFunc("/api/", apiHandler)
	http.ListenAndServe(":8080", nil)

}
