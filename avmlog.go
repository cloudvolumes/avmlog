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
	job_flag := flag.Bool("jobs", false, "Show background jobs")
	job_lines_flag := flag.Bool("job_lines", false, "Show matching lines from background jobs")
	sql_flag := flag.Bool("sql", false, "Show SQL statements")
	ntlm_flag := flag.Bool("ntlm", false, "Show NTLM lines")
	after_str := flag.String("after", "", "Show logs after this time (YYYY-MM-DD HH:II::SS")
	find_str := flag.String("find", "", "Find lines matching this regexp")

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

	fmt.Println(fmt.Sprintf("Show background jobs: %t", *job_flag))
	fmt.Println(fmt.Sprintf("Show lines from background jobs: %t", *job_lines_flag))
	fmt.Println(fmt.Sprintf("Show SQL: %t", *sql_flag))
	fmt.Println(fmt.Sprintf("After: %s", *after_str))

	filename := args[0]
	fmt.Println(fmt.Sprintf("Opening file: %s", filename))

	file := openFile(filename)
	defer file.Close()

	var reader io.Reader = file

	if isGzip(filename) {
		parse_gz_reader := getGzipReader(file)
		defer parse_gz_reader.Close()

		reader = parse_gz_reader
	}

	sql_regexp := regexp.MustCompile("(SQL \\()|(EXEC sp_executesql N)|( CACHE \\()")
	ntlm_regexp := regexp.MustCompile(" \\(NTLM\\) ")

	var unique_map map[string]bool;

	line_strexp := *find_str

	if line_regexp, err := regexp.Compile(line_strexp); len(line_strexp) > 0 && err == nil {
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
						if *job_flag || *job_lines_flag || !isJob(request_id) {
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
					if !*job_flag && isJob(request_id) {
            if *job_lines_flag {
							// if this is a job and jobs are hidden,
							// only print the line if job_lines is true and the line contains the original regexp
							output = has_matcher && line_regexp.MatchString(line)
						}
					} else {
						output = true
					}
				}
			} else {
				output = true
			}
		}

		if output {
			if !*sql_flag && sql_regexp.MatchString(line) {
				output = false
			} else if !*ntlm_flag && ntlm_regexp.MatchString(line) {
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
	flag.PrintDefaults()
	fmt.Println("Example: avm -find=\"username|computername\" \"/path/to/manager/log/production.log\"")
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