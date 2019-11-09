package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"os"
	"strconv"
)

var (
	serveHost = flag.String("serve_host", getEnv("SERVER_HOST", ""),
		"Host to serve requests incoming to Instagram Provider")
	servePort = flag.String("serve_port", getEnv("PORT", "8080"),
		"Port to serve requests incoming to Instagram Provider")
	g errgroup.Group

	voiceMail = VoiceMail{}
)

func main() {
	mainEndpoints := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", *serveHost, *servePort),
		Handler: handler(),
	}

	g.Go(func() error {
		return mainEndpoints.ListenAndServe()
	})

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getInt(strValue string, defaultValue int64) int64 {
	intValue, err := strconv.ParseInt(strValue, 10, 64)
	if err != nil {
		fmt.Printf("Incorrect int value, default value %+v will be used ", defaultValue)
		return defaultValue
	}
	return intValue
}

func Handler() func(func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return func(f func(w http.ResponseWriter, r *http.Request)) http.Handler {
		h := http.HandlerFunc(f)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		})
	}
}

func handler() http.Handler {
	r := mux.NewRouter()
	handler := Handler()
	r.Handle("/api/dialogs/voice-mail",
		handlers.LoggingHandler(
			os.Stdout,
			handler(voiceMail.handleRequest())),
	).Methods("POST")

	return JsonContentType(handlers.CompressHandler(r))
}

func JsonContentType(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		h.ServeHTTP(w, r)
	})
}
