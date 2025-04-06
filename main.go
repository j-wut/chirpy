package main

import (
	"fmt"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	server := &http.Server{
		Addr: ":8080",
		Handler: mux,
	}
	mux.Handle("/", http.FileServer(http.Dir("./site")))

	if err := server.ListenAndServe(); err != nil {
		fmt.Println(err) 
	}
}
