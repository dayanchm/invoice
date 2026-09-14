package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/dayanchm/invoice/internal/httpapi"
)

func main() {
	defaultAddress := "127.0.0.1:8080"
	if port := os.Getenv("PORT"); port != "" {
		defaultAddress = ":" + port
	}
	address := flag.String("addr", defaultAddress, "HTTP listen address")
	webDir := flag.String("web", "web", "browser application directory")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/invoice", httpapi.Handler)
	mux.Handle("/", http.FileServer(http.Dir(*webDir)))
	fmt.Printf("Invoice web app: http://%s\n", *address)
	log.Fatal(http.ListenAndServe(*address, mux))
}
