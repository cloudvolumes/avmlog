package main

import (
	"flag"
	"fmt"
	"time"
)

type parseOptions struct {
	hideJobsFlag  *bool
	hideSQLFlag   *bool
	hideNtlmFlag  *bool
	hideDebugFlag *bool
	onlyMsgFlag   *bool
	reportFlag    *bool
	fullFlag      *bool
	neatFlag      *bool
	detectErrors  *bool
	afterStr      *string
	findStr       *string
	hideStr       *string
	percent       *int
	metrics       *string
	fileName      string
	timeAfter     *time.Time
}

func (f *parseOptions) parseFlag() {
	f.hideJobsFlag = flag.Bool("hide_jobs", false, "Hide background jobs")
	f.hideSQLFlag = flag.Bool("hide_sql", false, "Hide SQL statements")
	f.hideNtlmFlag = flag.Bool("hide_ntlm", false, "Hide NTLM lines")
	f.hideDebugFlag = flag.Bool("hide_debug", false, "Hide DEBUG lines")
	f.onlyMsgFlag = flag.Bool("only_msg", false, "Output only the message portion")
	f.reportFlag = flag.Bool("report", false, "Collect request report")
	f.fullFlag = flag.Bool("full", false, "Show the full request/job for each found line")
	f.neatFlag = flag.Bool("neat", false, "Hide clutter - equivalent to -hide_jobs -hide_sql -hide_ntlm")
	f.detectErrors = flag.Bool("detect_errors", false, "Detect lines containing known error messages")
	f.afterStr = flag.String("after", "", "Show logs after this time (YYYY-MM-DD HH:II::SS")
	f.findStr = flag.String("find", "", "Find lines matching this regexp")
	f.hideStr = flag.String("hide", "", "Hide lines matching this regexp")
	f.percent = flag.Int("percent", 10, "how many cases (percentage) to use for report metrics")
	f.metrics = flag.String("metrics", "totalrequest", "Generate metrics based on which attributes")

	flag.Parse()
	args := flag.Args()
	checkArgs(args)
	f.fileName = args[0]
}

func (f *parseOptions) isNeatFlag() {
	if *f.neatFlag {
		*f.hideJobsFlag = true
		*f.hideSQLFlag = true
		*f.hideNtlmFlag = true
	}
}

func (f *parseOptions) printSelectedFlags() {
	msg(fmt.Sprintf("Show full requests/jobs: %t", *f.fullFlag))
	msg(fmt.Sprintf("Show background job lines: %t", !*f.hideJobsFlag))
	msg(fmt.Sprintf("Show SQL lines: %t", !*f.hideSQLFlag))
	msg(fmt.Sprintf("Show NTLM lines: %t", !*f.hideNtlmFlag))
	msg(fmt.Sprintf("Show DEBUG lines: %t", !*f.hideDebugFlag))
	msg(fmt.Sprintf("Show lines after: %s", *f.afterStr))
}
