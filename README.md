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

This tool uses the provided regexp to match target lines. 
Once these are identified, the request number "PxxxxRxxxx" from each of these target lines is extracted.
A second pass through the file is then performed and all log lines from those requests are printed.


## Platforms

- Windows: bin/avmlog.exe
- Linux: bin/avmlog


## Usage

You must pass a Manager log file and a regexp (start by using "user-name|computer-name")

avmlog [flags] [filename (manager/log/production.log)] [target-line-regex (user-name|computer-name)]

For example:
avmlog "/users/slawson/Documents/scale.log" "apvuser03734|av-pd1-pl8-0787"

### Flags:

#### -jobs=0|1

Controls whether or not background jobs are shown. 
The default is 0 because until the most recent development builds, 
all background jobs shared the same request identifier (PxxxxxDJ) so most of the output is not related.

In Manager builds (from master branch) created after October 21st, 
each background job has a unique request identifier (PxxxxDJxxx) so only relevant jobs can be shown. 

#### -sql=0/1

This controls whether or not SQL statements are shown. The default is 0 which strips them out.

#### -after="YYYY-MM-DD HH:II:SS"

Only include log lines from requests that occurred after this time.
This filter is used when determining which request identifiers (PxxxxxRxxxx) to include.
So it is possible that some lines from before this time will be printed if they belong to a request that crosses over.


## TODO

Things someone can do to improve this:

- Allow processing of multiple files (to account for load balanced Managers)
- Add ability to exclude log lines that occurred before a specific time
- Add ability to provide a custom exclusion regexp
- Add ability to specify user and computer directly instead of in a regexp
- Add ability to remove DEBUG lines
- Detect gaps in time of more than a few seconds for a single request and and print a line showing the time gap
- Write output to a file
- Write separate output files for each request identifier


## Developer notes

- Download go: https://golang.org/dl/
- Getting started: https://golang.org/doc/install
- Cross-platform compilation: git clone git://github.com/davecheney/golang-crosscompile.git

From: [workspace]/src/github.com/[your-account]/avmlog
- go install
- go-windows-386 build  (after doing: source golang-crosscompile/crosscompile.bash)