package main

// http://cefs.steve-meier.de/errata.latest.xml

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
)

type XMLOpt struct {
	Packages    []string `xml:"packages"`
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

func main() {
	file, err := ioutil.ReadFile("errata.latest.xml")
	if err != nil {
		panic(err)
	}
	v := XMLOpts{}
	err = xml.Unmarshal(file, &v)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}

	security := []XMLOpt{}
	for _, pkg := range v.Opt {
		if pkg.Type == "Security Advisory" {
			security = append(security, pkg)
		}
	}

	fmt.Println(security[1])

}
