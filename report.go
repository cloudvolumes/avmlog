package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	timeFormat     = "[2006-01-02 15:04:05 MST]"
	reportHeaders  = "RequestID, Method, URL, Computer, User, Request Result, Request Start, Request End,Total RequestTime (ms), Request Time (ms), Db Time (ms), View Time (ms), Mount Time (ms),vCenter Adapter time (ms), ESX adapter time (ms), Host total task time (ms), Host execution time (ms),  Total NTLM time,Session auth time (ms), Mount Type"
	compareHeaders = "Cases,,,,,,,,Total RequestTime (ms), Request Time (ms), Db Time (ms), View Time (ms), Mount Time (ms),vCenter Adapter time (ms), ESX adapter time (ms), Host total task time (ms), Host execution time (ms),,Session auth time (ms)"
)

var (
	timestampRegexp       = regexp.MustCompile("^(\\[[0-9-]+ [0-9:]+ UTC\\])")
	requestRegexp         = regexp.MustCompile("\\][[:space:]]+(P[0-9]+[A-Za-z]+[0-9]*) ")
	requestReconfigRegexp = regexp.MustCompile("\\][[:space:]]+(P[0-9]+[A-Za-z]+[0-9]*)+(RA || RS)+ ")
	resultRegexp          = regexp.MustCompile(" with result \\\"([a-z]+)\\\"")
	ntlmRegexp            = regexp.MustCompile(" (\\(NTLM\\)|NTLM:) ")
	ntlmStartRegexp       = regexp.MustCompile(" Authenticating URL ")
	ntlmEndRegexp         = regexp.MustCompile("NTLM authentication result:")
	vcAdapterRegexp       = regexp.MustCompile("Acquired 'vcenter' adapter ([0-9]+) of ([0-9]+) for '.*?' in ([0-9.]+)")
	esxAdapterRegexp      = regexp.MustCompile("Acquired 'esx' adapter ([0-9]+) of ([0-9]+) for '.*?' in ([0-9.]+)")
	computerRegexp        = regexp.MustCompile("workstation=(.*?)&")
	userRegexp            = regexp.MustCompile("username=(.*?)&")
	oldReconfigRegexp     = regexp.MustCompile(" RvSphere: Waking up in ReconfigVm#([a-z_]+) ")
	taskRegexp            = regexp.MustCompile("Task total time: ([0-9.]+)s \\(execution time ([0-9.]+)s\\)")
	sessionRegexp         = regexp.MustCompile(" NTLM authorization took: ([0-9.]+)ms")
	errorRregexp          = regexp.MustCompile("( ERROR | Exception | undefined | NilClass )")
	completeRegexp        = regexp.MustCompile(" Completed ([0-9]+) [A-Za-z ]+ in ([0-9.]+)ms \\(Views: ([0-9.]+)ms \\| ActiveRecord: ([0-9.]+)ms\\)")
	routeRegexp           = regexp.MustCompile(" INFO Started ([A-Z]+) \\\"\\/([-a-zA-Z0-9_/]+)\\?")
	reconfigRegexp        = regexp.MustCompile("Async completed for ([A-Z]+)+")
	mountTypeRegex        = regexp.MustCompile("Volumes will be mounted [A-Za-z]+")
	jobRegexp             = regexp.MustCompile("^P[0-9]+(DJ|PW)[0-9]*")
	sqlRegexp             = regexp.MustCompile("( SQL: | SQL \\()|(EXEC sp_executesql N)|( CACHE \\()")
	debugRegexp           = regexp.MustCompile(" DEBUG ")
	errorRegexp           = regexp.MustCompile("( ERROR | Exception | undefined | NilClass )")
	messageRegexp         = regexp.MustCompile(" P[0-9]+.*?[A-Z]+ (.*)")
	stripRegexp           = regexp.MustCompile("(_|-)?[0-9]+([_a-zA-Z0-9%!-]+)?")
	requestid             string
	reports               = map[string]*requestReport{}
	metricReports         = []*requestReport{}

	b []byte
)

type requestReport struct {
	step               int
	timeBeg            string
	timeEnd            string
	mount              float64
	method             string
	route              string
	computer           string
	user               string
	code               string
	requestTime        float64
	requestID          string
	db                 float64
	view               float64
	ntmlStart          string
	ntlmEnd            string
	session            float64
	totalNtlmTime      float64
	esxAdapterTime     float64
	vcenterAdapterTime float64
	hostTaskTime       float64
	hostExecutionTime  float64
	reconfigStart      string
	reconfigEnd        string
	totalReconfig      float64
	mountType          string
	totalRequestTime   float64
}

func extractRequestID(line string) string {
	if requestMatch := requestRegexp.FindStringSubmatch(line); len(requestMatch) > 1 {
		return requestMatch[1]
	}
	return ""

}

func extractRequest(line string) string {

	if requestMatch := requestReconfigRegexp.FindStringSubmatch(line); len(requestMatch) > 1 {
		returnString := strings.Replace(requestMatch[0], "]", "", 1)
		return strings.TrimSpace(returnString)
	}
	return ""

}

func generateRequestIDMap(requestIDS *[]string) map[string]bool {
	uniqueMap := make(map[string]bool, len(*requestIDS))

	for _, x := range *requestIDS {
		uniqueMap[x] = true
	}

	for k := range uniqueMap {
		msg(fmt.Sprintf("Request ID: %s", k))
	}

	return uniqueMap
}

func extractTimestamp(line string) string {
	if timestampMatch := timestampRegexp.FindStringSubmatch(line); len(timestampMatch) > 1 {
		return timestampMatch[1]
	}
	return ""

}

func isAfterTime(timestamp string, timeAfter *time.Time) bool {
	if lineTime, e := time.Parse(timeFormat, timestamp); e != nil {
		msg(fmt.Sprintf("Got error %s", e))
		return false
	} else if lineTime.Before(*timeAfter) {
		return false
	}

	return true
}

func isJob(requestID string) bool {
	return jobRegexp.MatchString(requestID)
}

func printReport() {
	fmt.Println(reportHeaders)

	for k, v := range reports {
		if v.code != "401" && len(v.code) > 0 {
			if !(v.mount > 0) {
				if v.totalReconfig > 0 {
					v.mount = (v.totalReconfig)
				} else {
					v.mount = v.hostTaskTime
				}
			}
			if v.mountType == "asynchronously" {
				v.totalRequestTime = v.requestTime + v.mount
			} else {
				v.totalRequestTime = v.requestTime
			}
			fmt.Println(fmt.Sprintf(
				"%s, %s, /%s, %s, %s, %s, %s, %s,%.2f, %.2f, %.2f, %.2f, %.2f, %.2f, %.2f, %.2f, %.2f,%0.2f,%.2f,%s",
				k,
				v.method,
				v.route,
				v.computer,
				v.user,
				v.code,
				v.timeBeg,
				v.timeEnd,
				v.totalRequestTime, //9
				v.requestTime,
				v.db,
				v.view,
				v.mount, //13
				v.vcenterAdapterTime,
				v.esxAdapterTime,
				v.hostTaskTime,
				v.hostExecutionTime,
				v.totalNtlmTime,
				v.session,
				v.mountType))

			if v.route == "user-login" {
				metricReports = append(metricReports, v)
			}
		}
	}
}

func extractKeyFields() {
	reader := bytes.NewReader(b)
	r := bufio.NewReader(reader)
	for {
		line, _, err := r.ReadLine()
		lineString := string(line[:])
		if err == io.EOF {
			break
		}
		report := &requestReport{}
		requestid := extractRequestID(lineString)
		requestExtracted := extractRequest(lineString)
		if len(requestid) > 0 || len(requestExtracted) > 0 {
			if timestamp := extractTimestamp(lineString); len(timestamp) > 1 {
				if routeLine := routeRegexp.FindStringSubmatch(lineString); len(routeLine) > 2 {
					report.route = routeLine[2]
					if computerLine := computerRegexp.FindStringSubmatch(lineString); len(computerLine) > 0 {
						report.computer = computerLine[1]
					}
					if userLine := userRegexp.FindStringSubmatch(lineString); len(userLine) > 0 {
						report.user = userLine[1]
					}
					report.timeBeg = timestamp
					report.requestID = requestid
					reports[requestid] = report
				} else {
					if strings.Contains(requestExtracted, requestid) {
						if len(requestid) == 0 && len(requestExtracted) > 0 {
							requestid = requestExtracted[:len(requestExtracted)-2]
						}
					}
					if report, ok := reports[requestid]; ok {
						if completeMatch := completeRegexp.FindStringSubmatch(lineString); len(completeMatch) > 1 {
							report.timeEnd = timestamp
							report.code = completeMatch[1]
							report.requestTime, _ = strconv.ParseFloat(completeMatch[2], 64)
							report.view, _ = strconv.ParseFloat(completeMatch[3], 64)
							report.db, _ = strconv.ParseFloat(completeMatch[4], 64)
						} else if reconfigLine := reconfigRegexp.FindStringSubmatch(lineString); len(reconfigLine) > 1 {
							mount := strings.Fields(lineString)[18]
							if len(mount) > 0 {
								report.mount, _ = strconv.ParseFloat(mount, 64)
								report.mount = report.mount * 1000
							}
						} else if ntlmLine := ntlmStartRegexp.FindStringSubmatch(lineString); len(ntlmLine) > 0 {
							report.ntmlStart = timestamp
						} else if mountTypeLine := mountTypeRegex.FindStringSubmatch(lineString); len(mountTypeLine) > 0 {
							report.mountType = strings.Fields(mountTypeLine[0])[4]
						} else if ntlmLine := ntlmEndRegexp.FindStringSubmatch(lineString); len(ntlmLine) > 0 {
							report.ntlmEnd = timestamp
							report.totalNtlmTime = timeDifference(report.ntmlStart, report.ntlmEnd)
						} else if sessionLine := sessionRegexp.FindStringSubmatch(lineString); len(sessionLine) > 0 {
							report.session, _ = strconv.ParseFloat(sessionLine[1], 64)
						} else if esxadapterline := esxAdapterRegexp.FindStringSubmatch(lineString); len(esxadapterline) > 2 {
							report.esxAdapterTime, _ = strconv.ParseFloat(esxadapterline[3], 64)
							report.esxAdapterTime = report.esxAdapterTime * 1000
						} else if vcadapterLine := vcAdapterRegexp.FindStringSubmatch(lineString); len(vcadapterLine) > 2 {
							report.vcenterAdapterTime, _ = strconv.ParseFloat(vcadapterLine[3], 64)
							report.vcenterAdapterTime = report.vcenterAdapterTime * 1000
						} else if hosttimeLine := taskRegexp.FindStringSubmatch(lineString); len(hosttimeLine) > 1 {
							report.hostTaskTime, _ = strconv.ParseFloat(hosttimeLine[1], 64)
							report.hostTaskTime = report.hostTaskTime * 1000
							report.hostExecutionTime, _ = strconv.ParseFloat(hosttimeLine[2], 64)
							report.hostExecutionTime = report.hostExecutionTime * 1000
						} else if reconfigmatch := oldReconfigRegexp.FindStringSubmatch(lineString); len(reconfigmatch) > 1 {
							if reconfigmatch[1] == "execute_task" {
								report.reconfigStart = timestamp
							} else if reconfigmatch[1] == "process_task" {
								report.reconfigEnd = timestamp
								report.totalReconfig = timeDifference(report.reconfigStart, report.reconfigEnd)
								report.totalReconfig = report.totalReconfig * 1000
							}
						}
					}
				}
			}
		}
	}
}
