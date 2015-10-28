# AppVolumes Manager Log Tool

This tool can be used to extract the logs for specific requests from an AppVolumes Manager log.

When troubleshooting a problem it is often necessary to correlate the following events:

* /pre-startup
* /computer-startup
* /user-login
* /user-logout
* /computer-shutdown

A human parsing usually searches for the username and/or computer name to identify these requests.
But once they are located, each line is often intermingled with other requests and difficult to follow.

This tool uses a regexp to match target lines. 
Once these target lines are identified, the request identifier "PxxxxRxxxx" from each of these target lines is extracted.
A second pass through the file is then performed and all log lines from those requests are printed.


## Binaries

These are available under the release section on github: https://github.com/cloudvolumes/avmlog/releases

- Windows: avmlog.exe
- Linux/Mac: avmlog


## Usage

Set operating flags and provide a AppVolumes Manager log file. Use gzipped files to save space.

`avmlog [flags] [filename]`

Find the full requests containing "apvuser2599" without background jobs, SQL, or NTLM lines:
`avmlog -find "apvuser2599" -full -neat ~/Documents/scale.log.gz`

Find lines containing "apvuser2599" after "2015-10-01 01:11:32" without SQL lines in a gzipped log file :
`avmlog -find="apvuser03734" -after="2015-10-01 01:11:32" -hide_sql "/users/slawson/Documents/scale.log.gz"`

Find lines containing a computer or user name:
`avmlog -find="apvuser03734|av-pd1-pl8-0787" "/users/slawson/Documents/scale.log.gz"`


### Flags:

#### -find="regexp"

The regular expression used to locate target lines.
If this is not provided, all lines will be printed unless excluded by other flags.

#### -hide="regexp"

The regular expression used to exclude lines.
This works only on the output phase.
It will not exclude an entire request from being included if a -find="xxx" -full was used.

#### -full

When a matching line is found, extract the request identifier from it (PxxxxxRxxxx),
and print all the lines in that request.

#### -neat

A shortcut for -hide_jobs, -hide_sql, and -hide_html.

#### -after="YYYY-MM-DD HH:II:SS"

Only include log lines from requests that occurred after this time.
This could be used to split large files like:
`avmlog -after="2015-10-20 06:24:12" "/users/slawson/Documents/scale.log" > scale_after_6-24pm.log`

#### -hide_jobs

Hide log lines from background jobs.

Until the most recent development builds all background jobs shared the same request identifier (PxxxxxDJ).
This is important to use with logs from those builds when using -full, 
otherwise lines from almost every job will be printed.

In Manager builds (in master branch) created after October 21st, 
each background job has a unique request identifier (PxxxxDJxxx) so only relevant jobs can be shown. 

#### -hide_sql

Hide log lines containing SQL statements.

#### -hide_ntlm

Hide log lines containing NTLM logs.

#### -hide_debug

Hide log lines containing DEBUG logs.


## TODO

Things someone can do to improve this:

- Allow processing of multiple files (to account for load balanced Managers)
- Add ability to specify user and computer directly instead of a -find=regexp
- Add ability to hide entire requests if a -hide="regexp" matches a line in that request (call it -hide_full)
- Detect gaps in time of more than a few seconds for a single request and and print a line showing the time gap
- Write separate output files for each request identifier
- Show the first line of each request
- Allow filtering of non-200 requests (maybe hide successful requests so errors are easier to find) 
- Add a context flag to pull back the X lines above/below a match
- Add flag to group requests so all their lines are together
- Add a flag to covert timestamps from UTC
- Add short-versions of the flags like -m/-a/-j etc
- Figure out how to use FlagSet: https://golang.org/pkg/flag/
- Improve progress bar


## Developer notes

- Download go: https://golang.org/dl/
- Getting started: https://golang.org/doc/install
- Cross-platform compilation: git clone git://github.com/davecheney/golang-crosscompile.git

From: [workspace]/src/github.com/[your-account]/avmlog
- go install
- go-windows-386 build  (after doing: source golang-crosscompile/crosscompile.bash)

Release names go in order alphabetically using the most liked name here:
http://www.superherodb.com/characters/