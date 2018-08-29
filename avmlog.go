package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
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
	var parseTime bool
	afterCount := 0
	timeAfter, err := time.Parse(timeFormat, fmt.Sprintf("[%s UTC]", *outputFlags.afterStr))
	if err != nil {
		if len(*outputFlags.afterStr) > 0 {
			msg(fmt.Sprintf("Invalid time format \"%s\" - Must be YYYY-MM-DD HH::II::SS", *outputFlags.afterStr))
			usage()
			os.Exit(2)
		}
	} else {
		parseTime = true
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
	file := openFile(filename)
	defer file.Close()

	isGzip := isGzip(filename)
	fileSize := float64(fileSize(file))
	showPercent := !isGzip

	reader = file

	if *outputFlags.detectErrors {
		*outputFlags.findStr = "( ERROR | Exception | undefined | Failed | NilClass | Unable | failed )"
	}

	findRegexp, err := regexp.Compile(*outputFlags.findStr)
	hasFind := len(*outputFlags.findStr) > 0 && err == nil

	hideRegexp, err := regexp.Compile(*outputFlags.hideStr)
	hasHide := len(*outputFlags.hideStr) > 0 && err == nil

	if *outputFlags.reportFlag || (*outputFlags.fullFlag && hasFind) {
		if isGzip {
			// for some reason if you create a reader but don't use it,
			// an error is given when the output reader is created below
			parseGzReader := getGzipReader(file)
			defer parseGzReader.Close()

			reader = parseGzReader
		}

		lineCount := 0
		lineAfter := !parseTime // if not parsing time, then all lines are valid
		requestIds := make([]string, 0)
		partialLine := false
		longLines := 0

		reader := bufio.NewReaderSize(reader, bufferSize)

		for {
			bytes, isPrefix, err := reader.ReadLine()

			line := string(bytes[:])

			if err == io.EOF {
				break
			}

			if err != nil {
				log.Fatal(err)
			}

			if isPrefix {
				if partialLine {
					continue
				} else {
					partialLine = true
					longLines++
				}
			} else {
				partialLine = false
			}

			if findRegexp.MatchString(line) {

				if !lineAfter {
					if timestamp := extractTimestamp(line); len(timestamp) > 1 {
						if isAfterTime(timestamp, &timeAfter) {
							lineAfter = true
							afterCount = lineCount
						}
					}
				}

				if lineAfter {
					if requestID := extractRequestID(line); len(requestID) > 1 {
						if !*outputFlags.hideJobsFlag || !isJob(requestID) {
							requestIds = append(requestIds, requestID)
						}
					}
				}
			} //find

			readSize += int64(len(line))

			if lineCount++; lineCount%20000 == 0 {
				if showPercent {
					showReadPercent(lineCount, float64(readSize)/fileSize, lineAfter, len(requestIds))
				} else {
					showBytes(lineCount, float64(readSize), lineAfter, len(requestIds))
				}
			}
		}

		fileSize = float64(readSize) // set the filesize to the total known size
		msg("")                      // empty line

		if longLines > 0 {
			msg(fmt.Sprintf("Warning: truncated %d long lines that exceeded %d bytes", longLines, bufferSize))
		}

		msg(fmt.Sprintf("Found %d lines matching \"%s\"", len(requestIds), *outputFlags.findStr))
		uniqueMap = generateRequestIDMap(&requestIds)

		if len(uniqueMap) < 1 {
			msg(fmt.Sprintf("Found 0 request identifiers for \"%s\"", *outputFlags.findStr))
			os.Exit(2)
		}

		rewindFile(file)
	} else {
		msg("Not printing -full requests, skipping request collection phase")
	}

	if isGzip {
		outputGZReader := getGzipReader(file)
		defer outputGZReader.Close()

		reader = outputGZReader
	}

	showPercent = readSize > int64(0)
	readSize = 0

	lineCount := 0
	lineAfter := !parseTime // if not parsing time, then all lines are valid
	hasRequests := len(uniqueMap) > 0
	inRequest := false

	outputReader := bufio.NewReaderSize(reader, bufferSize)

	for {
		bytes, _, err := outputReader.ReadLine()

		line := string(bytes[:])

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		output := false

		if !lineAfter {
			readSize += int64(len(line))

			if lineCount++; lineCount%5000 == 0 {
				if showPercent {
					fmt.Fprintf(os.Stderr, "Reading: %.2f%%\r", (float64(readSize)/fileSize)*100)
				} else {
					fmt.Fprintf(os.Stderr, "Reading: %d lines, %0.3f GB\r", lineCount, float64(readSize)/1024/1024/1024)
				}
			}

			if afterCount < lineCount {
				if timestamp := extractTimestamp(line); len(timestamp) > 1 {
					if isAfterTime(timestamp, &timeAfter) {
						msg("\n") // empty line
						lineAfter = true
					}
				}
			}
		}

		if lineAfter {
			requestID := extractRequestID(line)

			if hasRequests {
				if len(requestID) > 0 {
					if uniqueMap[requestID] {
						if *outputFlags.hideJobsFlag && isJob(requestID) {
							output = false
						} else {
							inRequest = true
							output = true
						}
					} else {
						inRequest = false
					}

				} else if len(requestID) < 1 && inRequest {
					output = true
				}
			} else if hasFind {
				output = findRegexp.MatchString(line)
			} else {
				output = true
			}
		}

		if output {
			if *outputFlags.hideSQLFlag && sqlRegexp.MatchString(line) {
				output = false
			} else if *outputFlags.hideNtlmFlag && ntlmRegexp.MatchString(line) {
				output = false
			} else if *outputFlags.hideDebugFlag && debugRegexp.MatchString(line) {
				output = false
			} else if hasHide && hideRegexp.MatchString(line) {
				output = false
			}
		}

		if output {
			if *outputFlags.onlyMsgFlag {
				if message_match := messageRegexp.FindStringSubmatch(line); len(message_match) > 1 {
					fmt.Println(stripRegexp.ReplaceAllString(strings.TrimSpace(message_match[1]), "***"))
				}
			} else {
				fmt.Println(line)
			}
		}
	}
}
