package server

import (
	"fmt"
	"interface/src/serial"
	"log"
	"net/http"
)

func SSEHandler(w http.ResponseWriter, r *http.Request, serialState serial.SerialState) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	for {
		data := <-serialState.Channel
		fmt.Fprintf(w, "%v", data)

		flusher.Flush()
	}
}

func StartServer(serialState *serial.SerialState) {
	http.HandleFunc("/data-streaming", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		for {
			data := <-serialState.Channel
			fmt.Fprintf(w, "%v", data)

			flusher.Flush()
		}
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "site/index.html")
	})

	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
