package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"flag"
)

func main() {
	filename := os.Args[1]
	fmt.Println(fmt.Sprintf("Opening file: %s", filename))

	job_flag := flag.Int("jobs", 0, "Show background jobs")
	sql_flag := flag.Int("sql", 0, "Show SQL statements")

	fmt.Println(fmt.Sprintf("Show background jobs: %d", *job_flag))
	fmt.Println(fmt.Sprintf("Show SQL: %d", *sql_flag))

	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	arg_regexp := os.Args[2]

	if len(arg_regexp) < 1 {
		fmt.Println("Usage: avmlog filename regexp")
		os.Exit(1)
	}

	line_count     := 0
	request_ids    := make([]string, 20)
	line_regexp    := regexp.MustCompile(arg_regexp) // ("apvuser03734|av-pd1-pl8-0787")
	request_regexp := regexp.MustCompile("\\] (P[0-9]+R[0-9]+) ")  // "\\] (P[0-9]+[A-Za-z]+[0-9]+) "
	if *job_flag > 0 {
		request_regexp = regexp.MustCompile("\\] (P[0-9]+[A-Za-z]+[0-9]+) ")  // "\\] (P[0-9]+[A-Za-z]+[0-9]+) "
	}
	sql_regexp     := regexp.MustCompile("(SQL \\()|(EXEC sp_executesql N)|( CACHE \\()")
	nltm_regexp    := regexp.MustCompile(" \\(NTLM\\) ")

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text();
		if line_regexp.MatchString(line) {
			request := request_regexp.FindStringSubmatch(line)

			if len(request) > 1 {
				request_ids = append(request_ids, request[1])
			}
		}

		line_count++

		if line_count % 10000 == 0 {
			fmt.Print(".")
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("about to unique things", len(request_ids))
	unique_set := make(map[string]bool, len(request_ids))

	for _, x := range request_ids {
		unique_set[x] = true
	}

	unique_ids := make([]string, 0, len(unique_set))

	for x := range unique_set {
		if len(x) > 0 {
			unique_ids = append(unique_ids, x)
		}
	}

	for i := 0; i < len(unique_ids); i++ {
		fmt.Println(fmt.Sprintf("Request ID: %s", unique_ids[i]))
	}

	unique_regexp := strings.Join(unique_ids, "|")
	fmt.Println(unique_regexp)

	if len(unique_regexp) < 1 {
		fmt.Println(fmt.Sprintf("Found 0 AVM Request IDs for %s", arg_regexp))
		os.Exit(2)
	}

	file.Seek(0, 0)

	output_regexp := regexp.MustCompile(unique_regexp)
	output_scanner := bufio.NewScanner(file)

	for output_scanner.Scan() {
		line := output_scanner.Text();
		if output_regexp.MatchString(line) {

			output := true

			if *sql_flag < 1 && sql_regexp.MatchString(line) {
				output = false
			}

			if nltm_regexp.MatchString(line) {
				output = false
			}

			if output {
				fmt.Println(line)
			}
		}
	}

	if err := output_scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
