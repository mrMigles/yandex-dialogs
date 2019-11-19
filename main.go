package main

import (
	"flag"
	"fmt"
	"github.com/azzzak/alice"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"os"
	"yandex-dialogs/common"
	"yandex-dialogs/masha"
	"yandex-dialogs/phrases_generator"
	"yandex-dialogs/voice_mail"
)

// 1. Implement your handler (dialog) complying this interface
type Dialog interface {

	// Returns func which takes incoming Alice request and prepared response with filled `session` information. Response should be returned.
	HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response

	// Returns related path of your dialog. For example, `api/dialogs/sample-dialog`
	GetPath() string
}

var (
	serveHost = flag.String("serve_host", common.GetEnv("SERVER_HOST", ""),
		"Host to serve requests incoming to Instagram Provider")
	servePort = flag.String("serve_port", common.GetEnv("PORT", "8080"),
		"Port to serve requests incoming to Instagram Provider")
	g errgroup.Group
)

// 2. Just add your implementation here
func buildHandlers() []Dialog {
	var dialogs []Dialog
	dialogs = append(dialogs, phrases_generator.NewDialog())
	dialogs = append(dialogs, voice_mail.VoiceMail{})
	dialogs = append(dialogs, masha.NewMasha())
	return dialogs
}

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

func handler() http.Handler {
	r := mux.NewRouter()
	handler := Handler()

	dialogs := buildHandlers()

	for _, v := range dialogs {
		r.Handle(v.GetPath(),
			handlers.LoggingHandler(
				os.Stdout,
				handler(handleRequest(v.HandleRequest()))),
		).Methods("POST", "OPTIONS")
	}

	return JsonContentType(handlers.CompressHandler(r))
}
