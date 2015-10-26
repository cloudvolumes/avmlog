package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"io"
	"regexp"
	"strings"
	"flag"
	"time"
	"compress/gzip"
)

const TIME_LAYOUT string = "[2006-01-02 15:04:05 MST]"
var job_regexp       *regexp.Regexp = regexp.MustCompile("^P[0-9]+(DJ|PW)[0-9]*")
var timestamp_regexp *regexp.Regexp = regexp.MustCompile("^(\\[[0-9-]+ [0-9:]+ UTC\\])")
var request_regexp   *regexp.Regexp = regexp.MustCompile("\\] (P[0-9]+[A-Za-z]+[0-9]+) ")

func main() {
	hide_jobs_flag := flag.Bool("hide_jobs", false, "Hide background jobs")
	hide_sql_flag  := flag.Bool("hide_sql", false, "Hide SQL statements")
	hide_ntlm_flag := flag.Bool("hide_ntlm", false, "Hide NTLM lines")
	full_flag      := flag.Bool("full", false, "Show the full request/job for each found line")
	neat_flag      := flag.Bool("neat", false, "Hide clutter - equivalent to -hide_jobs -hide_sql -hide_ntlm")
	after_str      := flag.String("after", "", "Show logs after this time (YYYY-MM-DD HH:II::SS")
	find_str       := flag.String("find", "", "Find lines matching this regexp")

	flag.Parse()
	args := flag.Args()

	// Time layouts must use the
	// reference time `Mon Jan 2 15:04:05 MST 2006` to show the
	// pattern with which to format/parse a given time/string

	time_after, e    := time.Parse(TIME_LAYOUT, fmt.Sprintf("[%s UTC]", *after_str))
	parse_time       := false

	if e != nil {
		if len(*after_str) > 0 {
			fmt.Println(fmt.Sprintf("Invalid time format \"%s\" - Must be YYYY-MM-DD HH::II::SS", *after_str))
			usage()
			os.Exit(2)
		}
	} else {
		parse_time = true
	}

	if len(args) < 1 {
		usage()
		os.Exit(2)
	}

	if *neat_flag {
		*hide_jobs_flag = true
		*hide_sql_flag = true
		*hide_ntlm_flag = true
	}

	fmt.Println(fmt.Sprintf("Show full requests/jobs: %t", *full_flag))
	fmt.Println(fmt.Sprintf("Show background job lines: %t", !*hide_jobs_flag))
	fmt.Println(fmt.Sprintf("Show SQL lines: %t", !*hide_sql_flag))
	fmt.Println(fmt.Sprintf("Show NTLM lines: %t", !*hide_ntlm_flag))
	fmt.Println(fmt.Sprintf("Show lines after: %s", *after_str))

	filename := args[0]
	fmt.Println(fmt.Sprintf("Opening file: %s", filename))

	file := openFile(filename)
	defer file.Close()

	var reader io.Reader = file

	sql_regexp := regexp.MustCompile("(SQL \\()|(EXEC sp_executesql N)|( CACHE \\()")
	ntlm_regexp := regexp.MustCompile(" \\(NTLM\\) ")

	var unique_map map[string]bool;

	line_strexp := *find_str

	if line_regexp, err := regexp.Compile(line_strexp); *full_flag && len(line_strexp) > 0 && err == nil {
		if isGzip(filename) {
			// for some reason if you create a reader but don't use it,
			// an error is given when the output reader is created below
			parse_gz_reader := getGzipReader(file)
			defer parse_gz_reader.Close()

			reader = parse_gz_reader
		}

		line_count  := 0
		line_after  := !parse_time // if not parsing time, then all lines are valid
		request_ids := make([]string, 0)

		scanner := bufio.NewScanner(reader);

		for scanner.Scan() {
			line := scanner.Text();
			if line_regexp.MatchString(line) {

				if !line_after {
					if timestamp := extractTimestamp(line); len(timestamp) > 1 {
						if isAfterTime(timestamp, &time_after) {
							line_after = true
						}
					}
				}

				if line_after {
					if request_id := extractRequestId(line); len(request_id) > 1 {
						if !*hide_jobs_flag || !isJob(request_id) {
							request_ids = append(request_ids, request_id)
						}
					}
				}
			}

			if line_count++; line_count % 10000 == 0 {
				fmt.Print(fmt.Sprintf("Reading: %d\r", line_count))
			}
		}

		fmt.Println("") // empty line

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		fmt.Println(fmt.Sprintf("Found %d lines matching \"%s\"", len(request_ids), line_strexp))
		unique_map = generateRequestIdMap(&request_ids)

		if len(unique_map) < 1 {
			fmt.Println(fmt.Sprintf("Found 0 request identifiers", line_strexp))
			os.Exit(2)
		}

		rewindFile(file)
	} else {
		fmt.Println("No matchers provided, skipping match phase")
	}

	if isGzip(filename) {
		output_gz_reader := getGzipReader(file)
		defer output_gz_reader.Close()

		reader = output_gz_reader
	}

	line_count := 0
	line_after := !parse_time // if not parsing time, then all lines are valid
	has_requests := len(unique_map) > 0

	line_regexp, err := regexp.Compile(line_strexp);
	has_matcher      := len(line_strexp) > 0 && err == nil

	output_scanner := bufio.NewScanner(reader);

	for output_scanner.Scan() {
		line := output_scanner.Text();

		output := false

		if !line_after {
			if line_count++; line_count % 10000 == 0 {
				fmt.Print(fmt.Sprintf("Reading: %d\r", line_count))
			}

			if timestamp := extractTimestamp(line); len(timestamp) > 1 {
				if isAfterTime(timestamp, &time_after) {
					fmt.Println("\n") // empty line
					line_after = true
				}
			}
		}

		if line_after {
			request_id := extractRequestId(line)

			if has_requests {
				if len(request_id) > 0 && unique_map[request_id] {
					if *hide_jobs_flag && isJob(request_id) {
						output = false
					} else {
						output = true
					}
				}
			} else if has_matcher {
				output = line_regexp.MatchString(line)
			} else {
				output = true
			}
		}

		if output {
			if *hide_sql_flag && sql_regexp.MatchString(line) {
				output = false
			} else if *hide_ntlm_flag && ntlm_regexp.MatchString(line) {
				output = false
			}
		}

		if output {
			fmt.Println(line)
		}
	}

	if err := output_scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Println("This tool can be used to extract the logs for specific requests from an AppVolumes Manager log")
	fmt.Println("")
	fmt.Println("Example:avmlog -after=\"2015-10-19 09:00:00\" -find \"apvuser2599\" -full -neat ~/Documents/scale.log.gz")
	fmt.Println("")
	flag.PrintDefaults()
	fmt.Println("")
}

func isAfterTime(timestamp string, time_after *time.Time) bool {
	if line_time, e := time.Parse(TIME_LAYOUT, timestamp); e != nil {
		fmt.Println("Got error %s", e)
		return false
	} else if line_time.Before(*time_after) {
		return false
	}

	return true
}

func isJob(request_id string) bool {
	return job_regexp.MatchString(request_id)
}

func extractTimestamp(line string) string {
	if timestamp_match := timestamp_regexp.FindStringSubmatch(line); len(timestamp_match) > 1 {
		return timestamp_match[1]
	} else {
		return ""
	}
}

func extractRequestId(line string) string {
	if request_match := request_regexp.FindStringSubmatch(line); len(request_match) > 1 {
		return request_match[1]
	} else {
		return ""
	}
}

func generateRequestIdMap(request_ids *[]string) map[string]bool {
	unique_map := make(map[string]bool, len(*request_ids))

	for _, x := range *request_ids {
		unique_map[x] = true
	}

	for k, _ := range unique_map {
		fmt.Println(fmt.Sprintf("Request ID: %s", k))
	}

	return unique_map
}

func openFile(filename string) *os.File {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	return file
}

func isGzip(filename string) bool {
	return strings.HasSuffix(filename, ".gz")
}

func getGzipReader(file *os.File) *gzip.Reader {
	gz_reader, err := gzip.NewReader(file)
	if err != nil {
	log.Fatal(err)
	}

	return gz_reader
}

func rewindFile(file *os.File) {
	file.Seek(0, 0)  // go back to the top (rewind)
}