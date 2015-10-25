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

func main() {
	job_flag := flag.Int("jobs", 0, "Show background jobs")
	sql_flag := flag.Int("sql", 0, "Show SQL statements")
	after_str := flag.String("after", "", "Show logs after this time (YYYY-MM-DD HH:II::SS")
	match_str := flag.String("match", "", "Regexp for requests to gather")

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
			os.Exit(4)
		}
	} else {
		parse_time = true
	}

	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	fmt.Println(fmt.Sprintf("Show background jobs: %d", *job_flag))
	fmt.Println(fmt.Sprintf("Show SQL: %d", *sql_flag))
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

	timestamp_regexp := regexp.MustCompile("^(\\[[0-9-]+ [0-9:]+ UTC\\])")
	sql_regexp       := regexp.MustCompile("(SQL \\()|(EXEC sp_executesql N)|( CACHE \\()")
	nltm_regexp      := regexp.MustCompile(" \\(NTLM\\) ")
	request_regexp   := regexp.MustCompile("\\] (P[0-9]+[A-Za-z]+[0-9]+) ")

	var unique_map map[string]bool;

	line_count  := 0
	line_after  := !parse_time // if not parsing time, then all lines are valid
	line_strexp := *match_str
	request_ids := make([]string, 0)

	if line_regexp, err := regexp.Compile(line_strexp); len(line_strexp) > 0 && err == nil {
		scanner := bufio.NewScanner(reader);

		for scanner.Scan() {
			line := scanner.Text();
			if line_regexp.MatchString(line) {

				if !line_after {
					if timestamp := timestamp_regexp.FindStringSubmatch(line); len(timestamp) > 1 {
						if is_after_time(&timestamp[1], &time_after) {
							line_after = true
						}
					}
				}

				if line_after {
					request := request_regexp.FindStringSubmatch(line)

					if len(request) > 1 {
						is_job := strings.Contains(request[1], "DJ")

						if is_job {
							if *job_flag > 0 {
								request_ids = append(request_ids, request[1])
							} else {
								//fmt.Println(line)
							}
						} else {
							request_ids = append(request_ids, request[1])
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

		file.Seek(0, 0)  // go back to the top (rewind)
	} else {
		fmt.Println("No matchers provided, skipping match phase")
	}

	if isGzip(filename) {
		output_gz_reader := getGzipReader(file)
		defer output_gz_reader.Close()

		reader = output_gz_reader
	}

	line_count = 0
	line_after = !parse_time // if not parsing time, then all lines are valid

	output_scanner := bufio.NewScanner(reader);

	for output_scanner.Scan() {
		line := output_scanner.Text();

		output := false

		if !line_after {
			if line_count++; line_count % 10000 == 0 {
				fmt.Print(fmt.Sprintf("Reading: %d\r", line_count))
			}

			if timestamp := timestamp_regexp.FindStringSubmatch(line); len(timestamp) > 1 {
				if is_after_time(&timestamp[1], &time_after) {
					fmt.Println("\n") // empty line
					line_after = true
				}
			}
		}

		if line_after {
			if request_id := request_regexp.FindStringSubmatch(line); len(request_id) > 1 {
				output = len(request_id[1]) > 0 && unique_map[request_id[1]]
			}
		}

		if output {
			if *sql_flag < 1 && sql_regexp.MatchString(line) {
				output = false
			} else if nltm_regexp.MatchString(line) {
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
	fmt.Println(fmt.Sprintf("Usage: avmlog -match=\"regexp\" -jobs=0|1 -sql=0|1 -after=\"YYYY-MM-DD HH:II::SS\" avmanager_filename.log"))
	fmt.Println("Example: avm -match=\"username|computername\" \"/path/to/manager/log/production.log\"")
}

func is_after_time(timestamp *string, time_after *time.Time) bool {
	if line_time, e := time.Parse(TIME_LAYOUT, *timestamp); e != nil {
		fmt.Println("Got error %s", e)
		return false
	} else if line_time.Before(*time_after) {
		return false
	}

	return true
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