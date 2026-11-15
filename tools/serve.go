//go:build ignore

// serve: web/ in a 960x540 iframe, like itch.io's, for live tests of the web build (sh tools/web.sh first).
// go run tools/serve.go [-addr localhost:8080], then open the page it prints. ?fps in the URL shows FPS/TPS.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "listen address")
	flag.Parse()
	files := http.FileServer(http.Dir("web"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/frame" {
			src := "/index.html" + "?" + r.URL.RawQuery
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<!doctype html><body style="margin:0;background:#222;display:grid;place-items:center;height:100vh"><iframe src="%s" width="960" height="540" style="border:0" allow="autoplay; fullscreen"></iframe>`, strings.ReplaceAll(src, `"`, "&quot;"))
			return
		}
		files.ServeHTTP(w, r) // game.wasm.gz as a plain file, no Content-Encoding: like itch.io
	})
	fmt.Printf("http://%s/ (add ?fps)\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
