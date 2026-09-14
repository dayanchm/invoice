package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/dayanchm/invoice/internal/httpapi"
)

//go:embed web
var webFiles embed.FS

func main() {
	public, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/invoice", httpapi.Handler)
	mux.Handle("/", http.FileServer(http.FS(public)))
	fmt.Printf("Invoice web app: http://127.0.0.1:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
