package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

var buildTime string
var startTime string
var envName string

func index(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<html><head><title>HELLO</title><meta http-equiv=\"refresh\" content=\"1\"></head>\n")

	fmt.Fprintf(w, "<body><h1>hello</h1>\n")
	fmt.Fprintf(w, "<table>\n")
	fmt.Fprintf(w, "<tr><td>env name</td><td>%s</td></tr>\n", envName)
	fmt.Fprintf(w, "<tr><td>build time</td><td>%s</td></tr>\n", buildTime)
	fmt.Fprintf(w, "<tr><td>start time</td><td>%s</td></tr>\n", startTime)
	fmt.Fprintf(w, "<tr><td>current time</td><td>%s</td></tr>\n", time.Now().Format("20060201-150405"))
	fmt.Fprintf(w, "</table></body>\n</html>")
}

func main() {
	envName = os.Getenv("NAME")

	startTime = time.Now().Format("20060201-150405")
	http.HandleFunc("/", index)
	http.ListenAndServe(":11180", nil)
}
