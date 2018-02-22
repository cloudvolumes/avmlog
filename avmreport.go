package main

import (
	"flag"
	"io/ioutil"
	"os"
	"sort"
	"strings"
)

var (
	file          *os.File
	err           error
	path          string
	percentReport int
	metricReport  string
)

func processReport() {

	args := flag.Args()
	base := checkIfZip(args[0])
	if len(base) > 0 {
		path = "production"
		filenames, err := unzip(args[0], "output")
		checkError("Unable to unzip", err)
		sort.Sort(sort.StringSlice(filenames))
		createOneLogFile(filenames)
		file, err = os.Open("output/production.log")
		checkError("Can not open file", err)

	} else {
		file, err = os.Open(args[0])
		checkError("Can not open file", err)
	}
	b, err = ioutil.ReadAll(file)
	extractKeyFields()
	printReport()
	allmetrics := strings.Split(metricReport, ",")
	for _, v := range allmetrics {
		createMetrics(v)
	}
	file.Close()
	if len(base) > 0 {
		os.RemoveAll("output")
	}
	os.Exit(0)
}
