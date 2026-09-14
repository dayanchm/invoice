package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	handler "github.com/dayanchm/invoice/api"
)

func main() {
	address := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	webDir := flag.String("web", "web", "browser application directory")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/invoice", handler.Handler)
	mux.Handle("/", http.FileServer(http.Dir(*webDir)))
	fmt.Printf("Invoice web app: http://%s\n", *address)
	log.Fatal(http.ListenAndServe(*address, mux))
}
