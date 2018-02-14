package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

// Time layouts must use the reference time `Mon Jan 2 15:04:05 MST 2006` to
// convey the pattern with which to format/parse a given time/string
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
	hideJobsFlag := flag.Bool("hide_jobs", false, "Hide background jobs")
	hideSQLFlag := flag.Bool("hide_sql", false, "Hide SQL statements")
	hideNtlmFlag := flag.Bool("hide_ntlm", false, "Hide NTLM lines")
	hideDebugFlag := flag.Bool("hide_debug", false, "Hide DEBUG lines")
	onlyMsgFlag := flag.Bool("only_msg", false, "Output only the message portion")
	reportFag := flag.Bool("report", false, "Collect request report")
	fullFlag := flag.Bool("full", false, "Show the full request/job for each found line")
	neatFlag := flag.Bool("neat", false, "Hide clutter - equivalent to -hide_jobs -hide_sql -hide_ntlm")
	detectErrors := flag.Bool("detect_errors", false, "Detect lines containing known error messages")
	afterStr := flag.String("after", "", "Show logs after this time (YYYY-MM-DD HH:II::SS")
	findStr := flag.String("find", "", "Find lines matching this regexp")
	hideStr := flag.String("hide", "", "Hide lines matching this regexp")
	percent := flag.Int("percent", 10, "how many cases (percentage) to use for report metrics")

	flag.Parse()
	args := flag.Args()

	timeAfter, err := time.Parse(timeFormat, fmt.Sprintf("[%s UTC]", *afterStr))
	parseTime := false
	afterCount := 0

	if err != nil {
		if len(*afterStr) > 0 {
			msg(fmt.Sprintf("Invalid time format \"%s\" - Must be YYYY-MM-DD HH::II::SS", *afterStr))
			usage()
			os.Exit(2)
		}
	} else {
		parseTime = true
	}

	if len(args) < 1 {
		usage()
		os.Exit(2)
	}

	if *neatFlag {
		*hideJobsFlag = true
		*hideSQLFlag = true
		*hideNtlmFlag = true
	}

	msg(fmt.Sprintf("Show full requests/jobs: %t", *fullFlag))
	msg(fmt.Sprintf("Show background job lines: %t", !*hideJobsFlag))
	msg(fmt.Sprintf("Show SQL lines: %t", !*hideSQLFlag))
	msg(fmt.Sprintf("Show NTLM lines: %t", !*hideNtlmFlag))
	msg(fmt.Sprintf("Show DEBUG lines: %t", !*hideDebugFlag))
	msg(fmt.Sprintf("Show lines after: %s", *afterStr))

	filename := args[0]
	msg(fmt.Sprintf("Opening file: %s", filename))
	if *reportFag {
		percentReport = *percent

		processReport()
	}
	file := openFile(filename)
	defer file.Close()

	isGzip := isGzip(filename)
	fileSize := float64(fileSize(file))
	showPercent := !isGzip

	reader = file

	if *detectErrors {
		*findStr = "( ERROR | Exception | undefined | Failed | NilClass | Unable | failed )"
	}

	findRegexp, err := regexp.Compile(*findStr)
	hasFind := len(*findStr) > 0 && err == nil

	hideRegexp, err := regexp.Compile(*hideStr)
	hasHide := len(*hideStr) > 0 && err == nil

	if *reportFag || (*fullFlag && hasFind) {
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
					longLines += 1
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
						if !*hideJobsFlag || !isJob(requestID) {
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

		msg(fmt.Sprintf("Found %d lines matching \"%s\"", len(requestIds), *findStr))
		uniqueMap = generateRequestIDMap(&requestIds)

		if len(uniqueMap) < 1 {
			msg(fmt.Sprintf("Found 0 request identifiers for \"%s\"", *findStr))
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
						if *hideJobsFlag && isJob(requestID) {
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
			if *hideSQLFlag && sqlRegexp.MatchString(line) {
				output = false
			} else if *hideNtlmFlag && ntlmRegexp.MatchString(line) {
				output = false
			} else if *hideDebugFlag && debugRegexp.MatchString(line) {
				output = false
			} else if hasHide && hideRegexp.MatchString(line) {
				output = false
			}
		}

		if output {
			if *onlyMsgFlag {
				if message_match := messageRegexp.FindStringSubmatch(line); len(message_match) > 1 {
					fmt.Println(stripRegexp.ReplaceAllString(strings.TrimSpace(message_match[1]), "***"))
				}
			} else {
				fmt.Println(line)
			}
		}
	}
}
