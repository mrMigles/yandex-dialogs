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
	"yandex-dialogs/phrases_generator"
	"yandex-dialogs/voice_mail"
)

var (
	serveHost = flag.String("serve_host", GetEnv("SERVER_HOST", ""),
		"Host to serve requests incoming to Instagram Provider")
	servePort = flag.String("serve_port", GetEnv("PORT", "8080"),
		"Port to serve requests incoming to Instagram Provider")
	g errgroup.Group

	voiceMail        = voice_mail.VoiceMail{}
	phrasesGenerator = phrases_generator.PhrasesGenerator{}
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

// Just add Handler here
func handler() http.Handler {
	r := mux.NewRouter()
	handler := Handler()

	// Voice Mail
	r.Handle("/api/dialogs/voice-mail",
		handlers.LoggingHandler(
			os.Stdout,
			handler(handleRequest(voiceMail.HandleRequest()))),
	).Methods("POST", "OPTIONS")

	// Phrases Generator
	r.Handle("/api/dialogs/phrases-generator",
		handlers.LoggingHandler(
			os.Stdout,
			handler(handleRequest(phrasesGenerator.HandleRequest()))),
	).Methods("POST", "OPTIONS")

	return JsonContentType(handlers.CompressHandler(r))
}
