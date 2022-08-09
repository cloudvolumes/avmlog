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
- Mac: avmlog
- Linux: avmlogx


## Usage

Set operating flags and provide a AppVolumes Manager log file. Use gzipped files to save space.

`avmlog [flags] [filename]`

Find the full requests containing "apvuser2599" without background jobs, SQL, or NTLM lines:

`avmlog -find "apvuser2599" -full -neat ~/Documents/scale.log.gz`

Find lines containing "apvuser2599" after "2015-10-01 01:11:32" without SQL lines in a gzipped log file :

`avmlog -find="apvuser03734" -after="2015-10-01 01:11:32" -hide_sql "/users/slawson/Documents/scale.log.gz"`

Find lines containing a computer or user name:

`avmlog -find="apvuser03734|av-pd1-pl8-0787" "/users/slawson/Documents/scale.log.gz"`

Get log messages without timestamps and numbers so similar messages can be sorted and counted:

`avmlog -full -find='Some!20!AppStack' -neat -only_msg scale.log.gz | sort -f | uniq -ic | sort -n`

sort -f = case-insensitive, uniq -ic = case-insensitive, add count, sort -n = natural number sort

> 386 RvSphere: Time taken to pop from wait queue *** milliseconds
> 
> 441 RvSphere: Preparing to reconfigure VM WSUAP***" (***) <running>
>
> 794 RvSphere: 		Task ReconfigVM_Task completed successfully
>
> 797 RvSphere: 	ReconfigVM_Task task: task***
>
> 806 RvSphere: 		Task total time: ***.*** (execution time ***.***)


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

#### -detect_errors

This is a shortcut flag to be used instead of -find to look for lines known to contain errors.
It is basically equivalent to -find="( ERROR | Exception | undefined | Failed | NilClass | Unable | failed )"

`avmlog -detect_errors -hide_sql "/users/slawson/Documents/scale.log"`

#### -only_msg

Show only the message portion of each log.
Removes the timestamp, process identifier, and log level from each line.

`avmlog -detect_errors -only_msg -hide_sql "/users/slawson/Documents/scale.log" | sort | uniq -c`

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

#### -report

Generates a comma-separated report containing one line for each request

`avmlog -report "/users/slawson/Documents/scale.log"`

### Mac
OSX will prevent you from running the binary after download because it is not signed, you can fix that by right-clicking it and choosing "Open". That will give you the dialog to "Open Anyway". 

Alternatively you can run this command:
`$ xattr -d com.apple.quarantine /path/to/avmlog`

## TODO

Things someone can do to improve this:

- Start making sub-directories and modules for the functions
-- https://linguinecode.com/post/how-to-import-local-files-packages-in-golang
- Allow processing of multiple files (to account for load balanced Managers)
- Add ability to specify user and computer directly instead of a -find=regexp
- Add ability to hide entire requests if a -hide="regexp" matches a line in that request (call it -hide_full)
- Add ability to extract multiple files from .zip files
- Add ability to group requests so all their lines are together
- Add ability to covert timestamps from UTC
- Add ability to pull back the X lines of context above and below a match
- Add ability to hide successful requests (so error are easier to find)
- Detect gaps in time of more than a few seconds for a single request and and print a line showing the time gap
- Write separate output files for each request identifier
- Show the first line and last line of each request 
- Add short-versions of the flags like -m/-a/-j etc
- Figure out how to use FlagSet: https://golang.org/pkg/flag/
- Improve progress bar
- Enable a file to be re-ordered based on time (good for merging 2+ logs)
- Add the ability to put a space after a line that matches a regexp
- Add a regexp list for common tasks


## Developer notes

### Compiling
- Download go: https://golang.org/dl/
- Getting started: https://golang.org/doc/install
- https://dave.cheney.net/2015/08/22/cross-compilation-with-go-1-5

From: [workspace]/src/github.com/[your-account]/avmlog
- go build 
- env GOOS=windows GOARCH=amd64 go build -v avmlog.go
- env GOOS=linux GOARCH=amd64 go build -v -o avmlogx avmlog.go
- env GOOS=darwin GOARCH=arm64 go build -v -o avmlogm avmlog.go

### Regex Debugging
https://regexr.com/
Backslashes need triple-escape because their in double-quotes
in .go: " INFO Started ([A-Z]+) \\\"\\/([-a-zA-Z0-9_/]+)\\?"
regexr: INFO Started ([A-Z]+) \"\/([-a-zA-Z0-9_/]+)(\?|\")

Single-quotes can be used to avoid escaping backslashes
Square brackets have to be escaped when using -find even with single-quotes

### Release Names
Release names go in order alphabetically using the most liked name here:
http://www.superherodb.com/characters/
https://www.superherodb.com/characters/?page_nr=42
