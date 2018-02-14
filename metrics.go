package main

import (
	"fmt"
	"log"
	"sort"
	"strconv"
)

var (
	sortedReports []*requestReport
	averageCase   int
)

func createMetrics() {
	for k, v := range reports {
		if v.route == "user-login" && v.code == "200" {
			sortedReports = append(sortedReports, reports[k])
		}
	}
	sort.Sort(reportSorter(sortedReports))

	totalSortedReports := len(sortedReports)
	averageCase = int((totalSortedReports * percentReport) / 100)

	if averageCase > totalSortedReports {
		log.Fatal("average given is greater than total records")
	}
	avgAll := "Avg. of all " + strconv.Itoa(totalSortedReports)
	fmt.Println(compareHeaders)
	calAvergage("Best case", sortedReports[:1])
	calAvergage("Avg. of Best "+strconv.Itoa(averageCase), sortedReports[:averageCase])
	calAvergage(avgAll, sortedReports[:])
	calAvergage("Avg. of worst "+strconv.Itoa(averageCase), sortedReports[totalSortedReports-averageCase:])
	calAvergage("Worst case", sortedReports[totalSortedReports-1:])

}

func calAvergage(testCase string, sortSlice []*requestReport) {
	atRequest := 0.0
	aRequest := 0.0
	aDB := 0.0
	aView := 0.0
	aMount := 0.0
	avAdapter := 0.0
	aeAdapter := 0.0
	aTotalTask := 0.0
	aExecTask := 0.0
	aSession := 0.0
	sliceLength := float64(len(sortSlice))
	for _, v := range sortSlice {
		atRequest += v.totalRequestTime
		aRequest += v.requestTime
		aDB += v.db
		aView += v.view
		aMount += v.mount
		avAdapter += v.vcenterAdapterTime
		aeAdapter += v.esxAdapterTime
		aTotalTask += v.hostTaskTime
		aExecTask += v.hostExecutionTime
		aSession += v.session
	}

	fmt.Println(fmt.Sprintf(
		"%s,%s,%s,%s,%s,%s,%s,%s,%.2f, %.2f, %.2f, %.2f, %.2f, %.2f, %.2f, %.2f, %.2f,%s,%.2f",
		testCase,
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		atRequest/sliceLength,
		aRequest/sliceLength,
		aDB/sliceLength,
		aView/sliceLength,
		aMount/sliceLength,
		avAdapter/sliceLength,
		aeAdapter/sliceLength,
		aTotalTask/sliceLength,
		aExecTask/sliceLength,
		"",
		aSession/sliceLength))
}
