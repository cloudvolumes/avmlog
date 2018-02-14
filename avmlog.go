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

//const REPORT_HEADERS = "RequestID, Method, URL, Computer, User, Request Result, Request Start, Request End, Request Time (ms), Db Time (ms), View Time (ms), Mount Time (ms), % Request Mounting, Mount Result, Errors, ESX-A, VC-A"

func main() {
	hide_jobs_flag := flag.Bool("hide_jobs", false, "Hide background jobs")
	hide_sql_flag := flag.Bool("hide_sql", false, "Hide SQL statements")
	hide_ntlm_flag := flag.Bool("hide_ntlm", false, "Hide NTLM lines")
	hide_debug_flag := flag.Bool("hide_debug", false, "Hide DEBUG lines")
	only_msg_flag := flag.Bool("only_msg", false, "Output only the message portion")
	report_flag := flag.Bool("report", false, "Collect request report")
	full_flag := flag.Bool("full", false, "Show the full request/job for each found line")
	neat_flag := flag.Bool("neat", false, "Hide clutter - equivalent to -hide_jobs -hide_sql -hide_ntlm")
	detect_errors := flag.Bool("detect_errors", false, "Detect lines containing known error messages")
	after_str := flag.String("after", "", "Show logs after this time (YYYY-MM-DD HH:II::SS")
	find_str := flag.String("find", "", "Find lines matching this regexp")
	hide_str := flag.String("hide", "", "Hide lines matching this regexp")
	percent := flag.Int("percent", 10, "how many cases (percentage) to use for report metrics")

	flag.Parse()
	args := flag.Args()

	time_after, err := time.Parse(timeFormat, fmt.Sprintf("[%s UTC]", *after_str))
	parse_time := false
	after_count := 0

	if err != nil {
		if len(*after_str) > 0 {
			msg(fmt.Sprintf("Invalid time format \"%s\" - Must be YYYY-MM-DD HH::II::SS", *after_str))
			Usage()
			os.Exit(2)
		}
	} else {
		parse_time = true
	}

	if len(args) < 1 {
		Usage()
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
	if *report_flag {
		percentReport = *percent

		processReport()
	}
	file := openFile(filename)
	defer file.Close()

	is_gzip := isGzip(filename)
	file_size := float64(fileSize(file))
	show_percent := !is_gzip
	var read_size int64 = 0

	var reader io.Reader = file
	var unique_map map[string]bool

	if *detect_errors {
		*find_str = "( ERROR | Exception | undefined | Failed | NilClass | Unable | failed )"
	}

	find_regexp, err := regexp.Compile(*find_str)
	has_find := len(*find_str) > 0 && err == nil

	hide_regexp, err := regexp.Compile(*hide_str)
	has_hide := len(*hide_str) > 0 && err == nil

	if *report_flag || (*full_flag && has_find) {
		if is_gzip {
			// for some reason if you create a reader but don't use it,
			// an error is given when the output reader is created below
			parse_gz_reader := getGzipReader(file)
			defer parse_gz_reader.Close()

			reader = parse_gz_reader
		}

		line_count := 0
		line_after := !parse_time // if not parsing time, then all lines are valid
		request_ids := make([]string, 0)
		partial_line := false
		long_lines := 0

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
				if partial_line {
					continue
				} else {
					partial_line = true
					long_lines += 1
				}
			} else {
				partial_line = false
			}

			if find_regexp.MatchString(line) {

				if !line_after {
					if timestamp := ExtractTimestamp(line); len(timestamp) > 1 {
						if isAfterTime(timestamp, &time_after) {
							line_after = true
							after_count = line_count
						}
					}
				}

				if line_after {
					if request_id := extractRequestID(line); len(request_id) > 1 {
						if !*hide_jobs_flag || !isJob(request_id) {
							request_ids = append(request_ids, request_id)
						}
					}
				}
			} //find

			read_size += int64(len(line))

			if line_count++; line_count%20000 == 0 {
				if show_percent {
					showPercent(line_count, float64(read_size)/file_size, line_after, len(request_ids))
				} else {
					ShowBytes(line_count, float64(read_size), line_after, len(request_ids))
				}
			}
		}

		file_size = float64(read_size) // set the filesize to the total known size
		msg("")                        // empty line

		if long_lines > 0 {
			msg(fmt.Sprintf("Warning: truncated %d long lines that exceeded %d bytes", long_lines, bufferSize))
		}

		msg(fmt.Sprintf("Found %d lines matching \"%s\"", len(request_ids), *find_str))
		unique_map = GenerateRequestIdMap(&request_ids)

		if len(unique_map) < 1 {
			msg(fmt.Sprintf("Found 0 request identifiers for \"%s\"", *find_str))
			os.Exit(2)
		}

		RewindFile(file)
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
	in_request := false

	output_reader := bufio.NewReaderSize(reader, bufferSize)

	for {
		bytes, _, err := output_reader.ReadLine()

		line := string(bytes[:])

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		output := false

		if !line_after {
			read_size += int64(len(line))

			if line_count++; line_count%5000 == 0 {
				if show_percent {
					fmt.Fprintf(os.Stderr, "Reading: %.2f%%\r", (float64(read_size)/file_size)*100)
				} else {
					fmt.Fprintf(os.Stderr, "Reading: %d lines, %0.3f GB\r", line_count, float64(read_size)/1024/1024/1024)
				}
			}

			if after_count < line_count {
				if timestamp := ExtractTimestamp(line); len(timestamp) > 1 {
					if isAfterTime(timestamp, &time_after) {
						msg("\n") // empty line
						line_after = true
					}
				}
			}
		}

		if line_after {
			request_id := extractRequestID(line)

			if has_requests {
				if len(request_id) > 0 {
					if unique_map[request_id] {
						if *hide_jobs_flag && isJob(request_id) {
							output = false
						} else {
							in_request = true
							output = true
						}
					} else {
						in_request = false
					}

				} else if len(request_id) < 1 && in_request {
					output = true
				}
			} else if has_find {
				output = find_regexp.MatchString(line)
			} else {
				output = true
			}
		}

		if output {
			if *hide_sql_flag && sqlRegexp.MatchString(line) {
				output = false
			} else if *hide_ntlm_flag && ntlmRegexp.MatchString(line) {
				output = false
			} else if *hide_debug_flag && debugRegexp.MatchString(line) {
				output = false
			} else if has_hide && hide_regexp.MatchString(line) {
				output = false
			}
		}

		if output {
			if *only_msg_flag {
				if message_match := messageRegexp.FindStringSubmatch(line); len(message_match) > 1 {
					fmt.Println(stripRegexp.ReplaceAllString(strings.TrimSpace(message_match[1]), "***"))
				}
			} else {
				fmt.Println(line)
			}
		}
	}
}
