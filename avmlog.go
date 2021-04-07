package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Time layouts must use the reference time `Mon Jan 2 15:04:05 MST 2006` to
// convey the pattern with which to format/parse a given time/string
const (
	TIME_LAYOUT string = "[2006-01-02 15:04:05 MST]"
	VERSION            = "v4.0.2 - Enigma"
	BUFFER_SIZE        = bufio.MaxScanTokenSize

	REPORT_HEADERS = "RequestID, Method, URL, Computer, User, Request Result, Request Start, Request End, Request Time (ms), Db Time (ms), View Time (ms), Mount Time (ms), % Request Mounting, Mount Result, Errors, ESX-A, VC-A"
)

var (
	job_regexp       *regexp.Regexp = regexp.MustCompile("^P[0-9]+(DJ|PW)[0-9]*")
	timestamp_regexp *regexp.Regexp = regexp.MustCompile("^(\\[[0-9-]+ [0-9:]+ UTC\\])")
	request_regexp   *regexp.Regexp = regexp.MustCompile("\\][[:space:]]+(P[0-9]+[A-Za-z]+[0-9]*) ")
	sql_regexp       *regexp.Regexp = regexp.MustCompile("( SQL: | SQL \\()|(EXEC sp_executesql N)|( CACHE \\()")
	ntlm_regexp      *regexp.Regexp = regexp.MustCompile(" (\\(NTLM\\)|NTLM:) ")
	debug_regexp     *regexp.Regexp = regexp.MustCompile(" DEBUG ")
	error_regexp     *regexp.Regexp = regexp.MustCompile("( ERROR | Exception | undefined | NilClass )")

	complete_regexp *regexp.Regexp = regexp.MustCompile(" Completed ([0-9]+) [A-Za-z ]+ in ([0-9.]+)ms \\(Views: ([0-9.]+)ms \\| ActiveRecord: ([0-9.]+)ms\\)")
	reconfig_regexp *regexp.Regexp = regexp.MustCompile(" RvSphere: Waking up in ReconfigVm#([a-z_]+) ")
	result_regexp   *regexp.Regexp = regexp.MustCompile(" with result \\\"([a-z]+)\\\"")
	route_regexp    *regexp.Regexp = regexp.MustCompile(" INFO Started ([A-Z]+) \\\"\\/([-a-zA-Z0-9_/]+)(\\?|\\\")")
	message_regexp  *regexp.Regexp = regexp.MustCompile(" P[0-9]+.*?[A-Z]+ (.*)")
	strip_regexp    *regexp.Regexp = regexp.MustCompile("(_|-)?[0-9]+([_a-zA-Z0-9%!-]+)?")
	computer_regexp *regexp.Regexp = regexp.MustCompile("workstation=(.*?)&")
	user_regexp     *regexp.Regexp = regexp.MustCompile("username=(.*?)&")

	vc_adapter_regexp  *regexp.Regexp = regexp.MustCompile("Acquired 'vcenter' adapter ([0-9]+) of ([0-9]+) for '.*?' in ([0-9.]+)")
	esx_adapter_regexp *regexp.Regexp = regexp.MustCompile("Acquired 'esx' adapter ([0-9]+) of ([0-9]+) for '.*?' in ([0-9.]+)")
)

type mount_report struct {
	queue        bool
	mount_beg    string
	mount_end    string
	mount_result string
	ms_mount     float64
}

type request_report struct {
	step          int
	time_beg      string
	time_end      string
	mounts        []*mount_report
	method        string
	route         string
	computer      string
	user          string
	code          string
	ms_request    float64
	ms_garbage    float64
	ms_db         float64
	ms_view       float64
	percent_mount int
	errors        int64
	vc_adapters   int64
	esx_adapters  int64
}

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

	flag.Parse()
	args := flag.Args()

	time_after, err := time.Parse(TIME_LAYOUT, fmt.Sprintf("[%s UTC]", *after_str))
	parse_time := false
	after_count := 0

	if err != nil {
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

	is_gzip := isGzip(filename)
	file_size := float64(fileSize(file))
	show_percent := !is_gzip
	var read_size int64 = 0

	var reader io.Reader = file
	var unique_map map[string]bool
	var reports = map[string]*request_report{}

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
		adapter_cnt := int64(0)
		partial_line := false
		long_lines := 0

		reader := bufio.NewReaderSize(reader, BUFFER_SIZE)

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
					if timestamp := extractTimestamp(line); len(timestamp) > 1 {
						if isAfterTime(timestamp, &time_after) {
							line_after = true
							after_count = line_count
						}
					}
				}

				if line_after {
					if request_id := extractRequestId(line); len(request_id) > 1 {
						if *report_flag {
							if !isJob(request_id) {
								if timestamp := extractTimestamp(line); len(timestamp) > 1 {
									if report, ok := reports[request_id]; ok {
										if error_regexp.MatchString(line) {
											report.errors += 1
										} else if vc_adapter_match := vc_adapter_regexp.FindStringSubmatch(line); len(vc_adapter_match) > 1 {
											adapter_cnt, _ = strconv.ParseInt(vc_adapter_match[1], 10, 64)
											if adapter_cnt > report.vc_adapters {
												report.vc_adapters = adapter_cnt
											}
										} else if esx_adapter_match := esx_adapter_regexp.FindStringSubmatch(line); len(esx_adapter_match) > 1 {
											adapter_cnt, _ = strconv.ParseInt(esx_adapter_match[1], 10, 64)
											if adapter_cnt > report.esx_adapters {
												report.esx_adapters = adapter_cnt
											}
										} else if reconfig_match := reconfig_regexp.FindStringSubmatch(line); len(reconfig_match) > 1 {
											if reconfig_match[1] == "execute_task" {
												report.step++
												report.mounts = append(report.mounts, &mount_report{mount_beg: timestamp, queue: true})
											} else if reconfig_match[1] == "process_task" {
												if report.step >= 0 {
													if mount := report.mounts[report.step]; mount != nil {
														if mount.queue {
															mount.mount_end = timestamp
															if result_match := result_regexp.FindStringSubmatch(line); len(result_match) > 1 {
																mount.mount_result = result_match[1]
															}
															mount_beg_time, _ := time.Parse(TIME_LAYOUT, mount.mount_beg)
															mount_end_time, _ := time.Parse(TIME_LAYOUT, mount.mount_end)
															mount.ms_mount = mount_end_time.Sub(mount_beg_time).Seconds() * 1000
														} else {
															msg("We got a process task with no execute task")
														}
													}
												}
											}
										} else if complete_match := complete_regexp.FindStringSubmatch(line); len(complete_match) > 1 {
											report.time_end = timestamp
											report.code = complete_match[1]

											report.ms_request, _ = strconv.ParseFloat(complete_match[2], 64)
											report.ms_view, _ = strconv.ParseFloat(complete_match[3], 64)
											report.ms_db, _ = strconv.ParseFloat(complete_match[4], 64)
										}
									} else {
										report := &request_report{step: -1, time_beg: timestamp}

										if route_match := route_regexp.FindStringSubmatch(line); len(route_match) > 1 {
											report.method = route_match[1]
											report.route = route_match[2]
										} else {
											msg("no matching route for new report! " + line)
										}

										if user_match := user_regexp.FindStringSubmatch(line); len(user_match) > 1 {
											report.user = user_match[1]
										}

										if computer_match := computer_regexp.FindStringSubmatch(line); len(computer_match) > 1 {
											report.computer = computer_match[1]
										}

										reports[request_id] = report
									}
								}
							}
						} else if !*hide_jobs_flag || !isJob(request_id) {
							request_ids = append(request_ids, request_id)
						}
					}
				}
			}

			read_size += int64(len(line))

			if line_count++; line_count%20000 == 0 {
				if show_percent {
					showPercent(line_count, float64(read_size)/file_size, line_after, len(request_ids))
				} else {
					showBytes(line_count, float64(read_size), line_after, len(request_ids))
				}
			}
		}

		file_size = float64(read_size) // set the filesize to the total known size
		msg("")                        // empty line

		if long_lines > 0 {
			msg(fmt.Sprintf("Warning: truncated %d long lines that exceeded %d bytes", long_lines, BUFFER_SIZE))
		}

		if len(reports) > 0 {
			fmt.Println(REPORT_HEADERS)

			for k, v := range reports {
				if len(v.method) > 0 && len(v.time_end) > 0 {
					var ms_mount float64

					for _, mount := range v.mounts {
						ms_mount += mount.ms_mount
					}

					fmt.Println(fmt.Sprintf(
						"%s, %s, /%s, %s, %s, %s, %s, %s, %.2f, %.2f, %.2f, %.2f, %.2f%%, %d, %d, %d, %d",
						k,
						v.method,
						v.route,
						v.computer,
						v.user,
						v.code,
						v.time_beg,
						v.time_end,
						v.ms_request,
						v.ms_db,
						v.ms_view,
						ms_mount,
						(ms_mount/v.ms_request)*100,
						len(v.mounts),
						v.errors,
						v.vc_adapters,
						v.esx_adapters))
				} else {
					msg("missing method or time_end for " + k)
				}
			}
			return
		}

		msg(fmt.Sprintf("Found %d lines matching \"%s\"", len(request_ids), *find_str))
		unique_map = generateRequestIdMap(&request_ids)

		if len(unique_map) < 1 {
			msg(fmt.Sprintf("Found 0 request identifiers for \"%s\"", *find_str))
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
	in_request := false

	output_reader := bufio.NewReaderSize(reader, BUFFER_SIZE)

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
				if timestamp := extractTimestamp(line); len(timestamp) > 1 {
					if isAfterTime(timestamp, &time_after) {
						msg("\n") // empty line
						line_after = true
					}
				}
			}
		}

		if line_after {
			request_id := extractRequestId(line)

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
			if *hide_sql_flag && sql_regexp.MatchString(line) {
				output = false
			} else if *hide_ntlm_flag && ntlm_regexp.MatchString(line) {
				output = false
			} else if *hide_debug_flag && debug_regexp.MatchString(line) {
				output = false
			} else if has_hide && hide_regexp.MatchString(line) {
				output = false
			}
		}

		if output {
			if *only_msg_flag {
				if message_match := message_regexp.FindStringSubmatch(line); len(message_match) > 1 {
					fmt.Println(strip_regexp.ReplaceAllString(strings.TrimSpace(message_match[1]), "***"))
				}
			} else {
				fmt.Println(line)
			}
		}
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

		return 1
	} else {
		msg(fmt.Sprintf("The file is %d bytes long", fi.Size()))

		return fi.Size()
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
	file.Seek(0, 0) // go back to the top (rewind)
}

func msg(output string) {
	fmt.Fprintln(os.Stderr, output)
}

func showPercent(line_count int, position float64, after bool, matches int) {
	fmt.Fprintf(
		os.Stderr,
		"Reading: %d lines, %.2f%% (after: %v, matches: %d)\r",
		line_count,
		position*100,
		after,
		matches)
}

func showBytes(line_count int, position float64, after bool, matches int) {
	fmt.Fprintf(
		os.Stderr,
		"Reading: %d lines, %0.3f GB (after: %v, matches: %d)\r",
		line_count,
		position/1024/1024/1024,
		after,
		matches)
}

// TODO: How to combine and re-order two (or more) log files
// open file1
// open file2
// loop start
// if file1 timestamp is blank - read file1 line, store line and it's timestamp
// if file2 timestamp is blank - read file2 line, store line and it's timestamp
// if file1 < file2
// print file1
// clear file1 timestamp
// else
// print file2
// clear file2 timestamp
//
