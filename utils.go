package main

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	prunnedFiles []string
)

//ReportSorter helps custom sort the struct
type ReportSorter []*RequestReport

//FileSorter helps custom sort the struct to sort based on Modtime
type FileSorter []*os.File

//Below 3  function are used for custom sorting
func (a ReportSorter) Len() int           { return len(a) }
func (a ReportSorter) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ReportSorter) Less(i, j int) bool { return a[i].totalRequestTime < a[j].totalRequestTime }

//FileExist chaecks if the filename given in argument exist if not will exit
func FileExist(args []string) bool {
	if len(args) < 1 {
		os.Exit(2)
	}
	return true
}

//CheckError will log fatal error and will exit
func CheckError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}

//TimeDifference will take two dates in string and return their diff in float64
func TimeDifference(a string, b string) float64 {
	diff := 0.0
	if a != "" && b != "" {
		startTime, _ := time.Parse(timeFormat, a)
		endTime, _ := time.Parse(timeFormat, b)
		diff = endTime.Sub(startTime).Seconds()
	}
	return diff
}

//RewindFile will seek to top of the file
func RewindFile(file *os.File) {
	file.Seek(0, 0)
}

//CheckIfZip will check if argument file is .log or .zip
func CheckIfZip(f string) string {
	base := filepath.Base(f)
	extension := filepath.Ext(f)
	if strings.ToLower(extension) == ".zip" {
		return basename(base)
	}
	return ""
}

func basename(s string) string {
	n := strings.LastIndexByte(s, '.')
	if n > 0 {
		return s[:n]
	}
	return s
}

//CreateOneLogFile will take slice of filenames and create one big file
func CreateOneLogFile(files []string) {
	f, err := os.Create("output/production.log")
	f, err = os.OpenFile("output/production.log", os.O_WRONLY|os.O_APPEND, 0644)

	CheckError("Failed opening file", err)
	defer f.Close()
	for _, v := range files {
		logFile, err := os.Open(v)
		CheckError("Failed opening file", err)
		content, err := ioutil.ReadAll(logFile)
		CheckError("Failed to read from ", err)
		f.Write(content)
		logFile.Close()
	}
}

// Unzip will un-compress a zip archive,
// moving all files and folders to an output directory specified by dest
func Unzip(src, dest string) ([]string, error) {

	var filenames []string

	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}
	defer r.Close()

	for _, f := range r.File {

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}
		defer rc.Close()

		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)
		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {

			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)

		} else {

			// Make File
			var fdir string
			if lastIndex := strings.LastIndex(fpath, string(os.PathSeparator)); lastIndex > -1 {
				fdir = fpath[:lastIndex]
			}

			err = os.MkdirAll(fdir, os.ModePerm)
			if err != nil {
				log.Fatal(err)
				return filenames, err
			}
			f, err := os.OpenFile(
				fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return filenames, err
			}
			defer f.Close()

			_, err = io.Copy(f, rc)
			if err != nil {
				return filenames, err
			}

		}
	}

	for _, v := range filenames {
		if strings.Contains(v, path) {
			prunnedFiles = append(prunnedFiles, v)
		}
	}

	return prunnedFiles, nil
}

//RemoveContents will delete the output folder created for log
func RemoveContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}
