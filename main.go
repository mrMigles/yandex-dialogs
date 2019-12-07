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

// 1. Implement your handler (dialog) complying this interface. Put implementation in separated folder/package.
type Dialog interface {
	// Returns func which takes incoming Alice request and prepared response with filled `session` information. Response should be returned.
	HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response

	// Returns base path of your dialog REST API. For example, `/api/dialogs/sample-dialog`
	GetPath() string

	// Returns state of dialog (true - ok, false - something is wrong) and additional string message.
	Health() (result bool, message string)
}

var (
	serveHost = flag.String("serve_host", common.GetEnv("SERVER_HOST", ""),
		"Host to serve requests incoming to server")
	servePort = flag.String("serve_port", common.GetEnv("PORT", "8080"),
		"Port to serve requests incoming to server")
	g errgroup.Group
)

// 2. Just add your implementation here
func buildHandlers() []Dialog {
	var dialogs []Dialog
	dialogs = append(dialogs, phrases_generator.NewDialog())
	dialogs = append(dialogs, voice_mail.NewVoiceMail())
	dialogs = append(dialogs, masha.NewMasha(2700))
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

	r.Handle("/health",
		handlers.LoggingHandler(
			os.Stdout,
			handler(handleHealthRequest(dialogs))),
	).Methods("GET")

	r.Handle("/statistics",
		handlers.LoggingHandler(
			os.Stdout,
			handler(handleStatisticsRequest())),
	).Methods("GET")

	return JsonContentType(handlers.CompressHandler(r))
}
