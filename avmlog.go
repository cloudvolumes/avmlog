package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"
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
	timeAfter, err := time.Parse(timeFormat, fmt.Sprintf("[%s UTC]", *outputFlags.afterStr))
	if err != nil {
		if len(*outputFlags.afterStr) > 0 {
			msg(fmt.Sprintf("Invalid time format \"%s\" - Must be YYYY-MM-DD HH::II::SS", *outputFlags.afterStr))
			usage()
			os.Exit(2)
		}
	} else {
		*outputFlags.timeAfter = timeAfter
	}

	outputFlags.isNeatFlag()
	outputFlags.printSelectedFlags()

	filename := outputFlags.fileName
	msg(fmt.Sprintf("Opening file: %s", filename))

	if *outputFlags.reportFlag {
		percentReport = *outputFlags.percent
		metricReport = *outputFlags.metrics

		processReport()
	}
	searchStr(outputFlags, filename)
}
