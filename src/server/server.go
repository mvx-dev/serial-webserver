package server

import (
	"fmt"
	"interface/src/serial"
	"log"
	"net/http"
)

func StartServer(serialState *serial.SerialState) {
	http.HandleFunc("/events-streaming", func(w http.ResponseWriter, r *http.Request) {
		data := <-serialState.Channel
		fmt.Fprintf(w, "%s", data)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "site/index.html")
	})

	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
