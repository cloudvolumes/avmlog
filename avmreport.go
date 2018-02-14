package main

import (
	"flag"
	"io/ioutil"
	"os"
	"sort"
)

var (
	file          *os.File
	err           error
	path          string
	percentReport int
)

func processReport() {

	args := flag.Args()
	base := CheckIfZip(args[0])
	if len(base) > 0 {
		path = "production"
		filenames, err := Unzip(args[0], "output")
		CheckError("Unable to unzip", err)
		sort.Sort(sort.StringSlice(filenames))
		CreateOneLogFile(filenames)
		file, err = os.Open("output/production.log")
		CheckError("Can not open file", err)

	} else {
		file, err = os.Open(args[0])
		CheckError("Can not open file", err)
	}
	b, err = ioutil.ReadAll(file)
	ExtractKeyFields()
	PrintReport()
	CreateMetrics()
	file.Close()
	if len(base) > 0 {
		os.RemoveAll("output")
	}
	os.Exit(0)
}
