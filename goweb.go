package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	markdown "github.com/gomarkdown/markdown"
	toml "github.com/pelletier/go-toml"
)

// thing or default
func boolOR(b interface{}, d bool) bool {
	if b != nil {
		return b.(bool)
	}

	return d
}

func stringOR(b interface{}, d string) string {
	if b != nil {
		return b.(string)
	}

	return d
}

// Load config
var config, _ = toml.LoadFile("./site.toml")

var staticPath string = stringOR(config.Get("static_path"), "./site")
var notFoundPath string = stringOR(config.Get("not_found_path"), "404.html")
var listenOn string = stringOR(config.Get("listen_on"), ":8080")
var markdownParse bool = boolOR(config.Get("parse_markdown"), true)

// Load file server
var fileServer = http.FileServer(http.Dir(staticPath))

// Middleware
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

func logware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := statusRecorder{w, 200}

		next.ServeHTTP(&rec, r)

		fmt.Printf("    <-- %v\n", rec.status)
	})
}

// Request handler
func _HandleRequest(w http.ResponseWriter, r *http.Request) {
	path, err := filepath.Abs(r.URL.Path)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	path = filepath.Join(staticPath, path)

	_, err = os.Stat(path)

	if os.IsNotExist(err) {
		// Serve 404 page if it exists, or just give 404 error with no content

		path = filepath.Join(staticPath, notFoundPath)

		_, err = os.Stat(path)

		if os.IsNotExist(err) {
			w.WriteHeader(404)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(404)
		body, err := ioutil.ReadFile(path)

		if err != nil {
			log.Fatalf("Unable to read file: %v", err)
		}

		w.Write(body)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Format markdown files
	if strings.HasSuffix(r.URL.Path, ".md") && markdownParse {
		// Read markdown file
		body, err := ioutil.ReadFile(path)

		if err != nil {
			log.Fatalf("Unable to read file: %v", err)
		}

		// Read template from /_markdown.html
		template, err := ioutil.ReadFile(filepath.Join(staticPath, "_markdown.html"))

		if err != nil {
			log.Fatalf("Unable to read file: %v", err)
		}

		// Use template and generate HTML page from parsed markdown
		data := []byte(fmt.Sprintf(string(template), string(markdown.ToHTML(body, nil, nil))))

		w.Write(data)
		return
	}

	// Serve requested file
	fileServer.ServeHTTP(w, r)
}

// Middleware request handler
func HandleRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("%s --> %s %s %s \n", r.RemoteAddr, r.Method, r.Host, r.URL.Path)
	_HandleRequest(w, r)
}

// Main function
func main() {
	fmt.Printf("Running web server serving %s at %s\n", staticPath, listenOn)

	http.ListenAndServe(listenOn, logware(http.HandlerFunc(HandleRequest)))
}
