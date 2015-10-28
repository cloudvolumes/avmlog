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

// Time layouts must use the reference time `Mon Jan 2 15:04:05 MST 2006` to
// convey the pattern with which to format/parse a given time/string
const TIME_LAYOUT string = "[2006-01-02 15:04:05 MST]"
const VERSION = "v2.1 - Bizarro"

var job_regexp       *regexp.Regexp = regexp.MustCompile("^P[0-9]+(DJ|PW)[0-9]*")
var timestamp_regexp *regexp.Regexp = regexp.MustCompile("^(\\[[0-9-]+ [0-9:]+ UTC\\])")
var request_regexp   *regexp.Regexp = regexp.MustCompile("\\] (P[0-9]+[A-Za-z]+[0-9]+) ")
var sql_regexp       *regexp.Regexp = regexp.MustCompile("(SQL \\()|(EXEC sp_executesql N)|( CACHE \\()")
var ntlm_regexp      *regexp.Regexp = regexp.MustCompile(" \\(NTLM\\) ")
var debug_regexp     *regexp.Regexp = regexp.MustCompile(" DEBUG ")

func main() {
	hide_jobs_flag  := flag.Bool("hide_jobs", false, "Hide background jobs")
	hide_sql_flag   := flag.Bool("hide_sql", false, "Hide SQL statements")
	hide_ntlm_flag  := flag.Bool("hide_ntlm", false, "Hide NTLM lines")
	hide_debug_flag := flag.Bool("hide_debug", false, "Hide DEBUG lines")
	full_flag       := flag.Bool("full", false, "Show the full request/job for each found line")
	neat_flag       := flag.Bool("neat", false, "Hide clutter - equivalent to -hide_jobs -hide_sql -hide_ntlm")
	after_str       := flag.String("after", "", "Show logs after this time (YYYY-MM-DD HH:II::SS")
	find_str        := flag.String("find", "", "Find lines matching this regexp")

	flag.Parse()
	args := flag.Args()

	time_after, e    := time.Parse(TIME_LAYOUT, fmt.Sprintf("[%s UTC]", *after_str))
	parse_time       := false

	if e != nil {
		if len(*after_str) > 0 {
			msg(fmt.Sprintf("Invalid time format \"%s\" - Must be YYYY-MM-DD HH::II::SS", *after_str))
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

	msg(fmt.Sprintf("Show full requests/jobs: %t", *full_flag))
	msg(fmt.Sprintf("Show background job lines: %t", !*hide_jobs_flag))
	msg(fmt.Sprintf("Show SQL lines: %t", !*hide_sql_flag))
	msg(fmt.Sprintf("Show NTLM lines: %t", !*hide_ntlm_flag))
	msg(fmt.Sprintf("Show DEBUG lines: %t", !*hide_debug_flag))
	msg(fmt.Sprintf("Show lines after: %s", *after_str))

	filename := args[0]
	msg(fmt.Sprintf("Opening file: %s", filename))

	file := openFile(filename)
	defer file.Close()

	is_gzip      := isGzip(filename)
	file_size    := float64(fileSize(file))
	show_percent := !is_gzip
	var read_size int64 = 0

	var reader io.Reader = file
	var unique_map map[string]bool;

	line_strexp := *find_str

	if line_regexp, err := regexp.Compile(line_strexp); *full_flag && len(line_strexp) > 0 && err == nil {
		if is_gzip {
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

			read_size += int64(len(line))

			if line_count++; line_count % 20000 == 0 {
				if show_percent {
					showPercent(line_count, float64(read_size) / file_size, line_after, len(request_ids))
				} else {
					showBytes(line_count, float64(read_size), line_after, len(request_ids))
				}
			}
		}

		file_size = float64(read_size) // set the filesize to the total known size
		msg("") // empty line

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		msg(fmt.Sprintf("Found %d lines matching \"%s\"", len(request_ids), line_strexp))
		unique_map = generateRequestIdMap(&request_ids)

		if len(unique_map) < 1 {
			msg(fmt.Sprintf("Found 0 request identifiers for \"%s\"", line_strexp))
			os.Exit(2)
		}

		rewindFile(file)
	} else {
		msg("Not printing -full requests, skipping request collection phase")
	}

	if is_gzip {
		output_gz_reader := getGzipReader(file)
		defer output_gz_reader.Close()

		reader = output_gz_reader
	}

	show_percent = read_size > int64(0)
	read_size = 0

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
			read_size += int64(len(line))

			if line_count++; line_count % 5000 == 0 {
				if show_percent {
					fmt.Fprintf(os.Stderr, "Reading: %.2f%%\r", (float64(read_size) / file_size) * 100)
				} else {
					fmt.Fprintf(os.Stderr, "Reading: %d lines, %0.3f GB\r", line_count, float64(read_size)/1024/1024/1024)
				}
			}

			if timestamp := extractTimestamp(line); len(timestamp) > 1 {
				if isAfterTime(timestamp, &time_after) {
					msg("\n") // empty line
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
			} else if *hide_debug_flag && debug_regexp.MatchString(line) {
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
	msg("AppVolumes Manager Log Tool - " + VERSION)
	msg("This tool can be used to extract the logs for specific requests from an AppVolumes Manager log")
	msg("")
	msg("Example:avmlog -after=\"2015-10-19 09:00:00\" -find \"apvuser2599\" -full -neat ~/Documents/scale.log.gz")
	msg("")
	flag.PrintDefaults()
	msg("")
}

func isAfterTime(timestamp string, time_after *time.Time) bool {
	if line_time, e := time.Parse(TIME_LAYOUT, timestamp); e != nil {
		msg(fmt.Sprintf("Got error %s", e))
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
		msg(fmt.Sprintf("Request ID: %s", k))
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

func fileSize(file *os.File) int64 {
	if fi, err := file.Stat(); err != nil {
		msg("Unable to determine file size")

		return 1;
	} else {
		msg(fmt.Sprintf("The file is %d bytes long", fi.Size()))

		return fi.Size();
	}
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

func msg(output string) {
	fmt.Fprintln(os.Stderr, output)
}

func showPercent(line_count int, position float64, after bool, matches int) {
	fmt.Fprintf(
		os.Stderr,
		"Reading: %d lines, %.2f%% (after: %v, matches: %d)\r",
		line_count,
		position * 100,
		after,
		matches)
}

func showBytes(line_count int, position float64, after bool, matches int) {
	fmt.Fprintf(
		os.Stderr,
		"Reading: %d lines, %0.3f GB (after: %v, matches: %d)\r",
		line_count,
		position / 1024 / 1024 / 1024,
		after,
		matches)
}