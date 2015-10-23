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

Windows: bin/avmlog.exe
Linux: bin/avmlog


## Usage

avmlog [flags] [avmanager_filename.log] [target-line-regexp]

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


## Developer notes

- Download go: https://golang.org/dl/
- Getting started: https://golang.org/doc/install
- Cross-platform compilation: git clone git://github.com/davecheney/golang-crosscompile.git

From: <workspace>/src/github.com/<your-account>/avmlog
- go install
- go-windows-386 build  (after doing: source golang-crosscompile/crosscompile.bash)