package main

import (
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

func processReport(filename string) {

	file, base := getLogFile(filename)
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
}

func getLogFile(filename string) (*os.File, string) {
	base := checkIfZip(filename)
	if len(base) > 0 {
		path = "production"
		filenames, err := unzip(filename, "output")
		checkError("Unable to unzip", err)
		sort.Sort(sort.StringSlice(filenames))
		createOneLogFile(filenames)
		file, err = os.Open("output/production.log")
		checkError("Can not open file", err)

	} else {
		file, err = os.Open(filename)
		checkError("Can not open file", err)
	}

	return file, base
}
