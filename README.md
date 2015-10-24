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


## Platforms

- Windows: bin/avmlog.exe
- Linux: bin/avmlog


## Usage

Set operating flags and provide a AppVolumes Manager log file. Use gzipped files to save space.

avmlog [flags] [filename]

For example:
avmlog -match="apvuser03734|av-pd1-pl8-0787" "/users/slawson/Documents/scale.log"

With a gzipped file:
avmlog -match="apvuser03734|av-pd1-pl8-0787" -after="2015-10-01 01:11:32" "/users/slawson/Documents/scale.log.gz"


### Flags:

#### -match="regexp"

The regular expression used to locate target lines during the first pass.

#### -after="YYYY-MM-DD HH:II:SS"

Only include log lines from requests that occurred after this time.
This could be used to split large files
avmlog -after="2015-10-20 06:24:12" -sql=1 "/users/slawson/Documents/scale.log" > scale_after_6-24pm.log

#### -jobs=0|1

Controls whether or not background jobs are shown. 
The default is 0 because until the most recent development builds, 
all background jobs shared the same request identifier (PxxxxxDJ) so most of the output is not related.

In Manager builds (from master branch) created after October 21st, 
each background job has a unique request identifier (PxxxxDJxxx) so only relevant jobs can be shown. 

#### -sql=0/1

This controls whether or not SQL statements are shown. The default is 0 which strips them out.


## TODO

Things someone can do to improve this:

- Allow processing of multiple files (to account for load balanced Managers)
- Add ability to provide a custom exclusion regexp
- Add ability to specify user and computer directly instead of in a regexp
- Add ability to remove DEBUG lines
- Detect gaps in time of more than a few seconds for a single request and and print a line showing the time gap
- Write separate output files for each request identifier


## Developer notes

- Download go: https://golang.org/dl/
- Getting started: https://golang.org/doc/install
- Cross-platform compilation: git clone git://github.com/davecheney/golang-crosscompile.git

From: [workspace]/src/github.com/[your-account]/avmlog
- go install
- go-windows-386 build  (after doing: source golang-crosscompile/crosscompile.bash)

Release names go in order alphabetically using the most liked name here:
http://www.superherodb.com/characters/