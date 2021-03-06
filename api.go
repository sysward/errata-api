package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

import _ "net/http/pprof"

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

var mutex sync.RWMutex
var lastModified time.Time

func ShouldRefreshErrata() bool {
	resp, err := http.Head("http://cefs.steve-meier.de/errata.latest.xml")
	if err != nil {
		fmt.Println("[~] Errata HEAD failed")
		return false
	}

	defer resp.Body.Close()

	// Example: Fri, 31 Oct 2014 09:40:46 GMT
	const longForm = "Mon, 02 Jan 2006 03:04:05 MST"
	_lastModified, err := time.Parse(longForm, resp.Header.Get("Last-Modified"))
	if err != nil {
		fmt.Println("[~] Time Parse failed: ", resp.Header.Get("Last-Modified"))
		return false
	}

	// Dont compare Time objects, make sure they're something comparible first.
	//if fmt.Sprintf("%s", _lastModified) != fmt.Sprintf("%s", lastModified) {
	if !_lastModified.Equal(lastModified) {
		fmt.Println(fmt.Sprintf("%s", _lastModified), "=", fmt.Sprintf("%s", lastModified))
		lastModified = _lastModified
		return true
	} else {
		return false
	}
}

func GetSecurityErrata() []byte {
	// http://cefs.steve-meier.de/errata.latest.xml
	resp, err := http.Get("http://cefs.steve-meier.de/errata.latest.xml")
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	return body
}

func ParsePackageVersion(name string) int {
	if !strings.Contains(name, "-") {
		return 0
	}
	parts := strings.Split(name, "-")
	version := strings.Split(parts[1], ".")
	majorVersion, _ := strconv.ParseInt(version[0], 10, 0)
	return int(majorVersion)
}

func ParseSecurityErrata() {
	v := XMLOpts{}
	errata := GetSecurityErrata()
	versionLUT = map[int]packageLUT{}
	err := xml.Unmarshal(errata, &v)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}

	for _, pkg := range v.Opt {
		if pkg.Type == "Security Advisory" {
			for _, ver := range pkg.OsRelease {
				if versionLUT[ver] == nil {
					versionLUT[ver] = packageLUT{}
				}
				for _, pack := range pkg.Packages {
					major := ParsePackageVersion(pack)

					if versionLUT[ver][major] == nil {
						versionLUT[ver][major] = []XMLOpt{}
					}

					versionLUT[ver][major] = append(versionLUT[ver][major], pkg)
				}
			}
		}
	}
}

func CheckForUpdates() {
	mutex.Lock()
	if ShouldRefreshErrata() {
		fmt.Println("!!!![ ]!!!! Refreshing errata....", time.Now())
		ParseSecurityErrata()
		fmt.Println("!!!![x]!!!! Refreshing errata....", time.Now())
	}
	mutex.Unlock()
}

type packageLUT map[int][]XMLOpt

var versionLUT map[int]packageLUT = map[int]packageLUT{}

func AppendIfMissing(slice []XMLOpt, x XMLOpt) []XMLOpt {
	for _, ele := range slice {
		if ele.Equal(x) {
			return slice
		}
	}
	return append(slice, x)
}

func (p *XMLOpt) Equal(o XMLOpt) bool {
	if p.Release != o.Release {
		return false
	}
	if len(p.OsRelease) != len(o.OsRelease) {
		return false
	}
	for i, pr := range p.OsRelease {
		if o.OsRelease[i] != pr {
			return false
		}
	}
	if len(p.Packages) != len(o.Packages) {
		return false
	}
	for i, pp := range p.Packages {
		if o.Packages[i] != pp {
			return false
		}
	}
	return true
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/"):]
	pathArr := strings.Split(path, "/")
	_, osRelease, pkg, pkgVersion := pathArr[1], pathArr[2], pathArr[3], pathArr[4]

	majorVer := strings.Split(pkgVersion, ".")[0]
	version, _ := strconv.ParseInt(osRelease, 10, 0)
	release, _ := strconv.ParseInt(majorVer, 10, 0)

	//fmt.Println("pkgVersion =", pkgVersion)
	//fmt.Println("osRelease =", osRelease)
	//fmt.Println("majorVer =", majorVer)
	//fmt.Println("pkg =", pkg)

	mutex.RLock()
	defer mutex.RUnlock()
	xpkgs := versionLUT[int(version)][int(release)]
	respPkgs := []XMLOpt{}
	for _, xpkg := range xpkgs {
		for _, vpkg := range xpkg.Packages {
			if strings.Contains(vpkg, pkg) && strings.Contains(vpkg, pkgVersion) {
				respPkgs = AppendIfMissing(respPkgs, xpkg)
			}
		}
	}
	err := json.NewEncoder(w).Encode(respPkgs)
	if err != nil {
		fmt.Println(err)
	}
}

func apiUpdatedAt(w http.ResponseWriter, r *http.Request) {
	resp := struct {
		UpdatedAt time.Time
	}{lastModified}

	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		fmt.Println(err)
	}
}

func main() {

	CheckForUpdates()

	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for t := range ticker.C {
			fmt.Println("[ ] Checking for updates....", t)
			CheckForUpdates()
			fmt.Println("[x] Checking for updates....", t)
		}
	}()

	http.HandleFunc("/api/updated", apiUpdatedAt)
	http.HandleFunc("/api/", apiHandler)
	err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		panic(err)
	}

}
