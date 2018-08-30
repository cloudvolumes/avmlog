package main

import (
	"bufio"
	"io"
)

const (
	version    = "v4.0.0 - Elektra"
	bufferSize = bufio.MaxScanTokenSize
)

var (
	readSize  int64
	uniqueMap map[string]bool
	reader    io.Reader
)

func main() {
	outputFlags := &parseOptions{}
	outputFlags.parseFlag()
	outputFlags.isNeatFlag()
	outputFlags.printSelectedFlags()
	filename := outputFlags.fileName
	if *outputFlags.reportFlag {
		percentReport = *outputFlags.percent
		metricReport = *outputFlags.metrics
		processReport(filename)
	} else {
		searchStr(outputFlags, filename)
	}
}
