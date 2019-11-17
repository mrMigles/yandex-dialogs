package phrases_generator

import (
	"github.com/azzzak/alice"
	"net/http"
	"sync"
)

var client = http.Client{}

type PhrasesGenerator struct {
	states map[string]string
	mux    sync.Mutex
}

func (v PhrasesGenerator) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) *alice.Response {
		v.mux.Lock()
		defer v.mux.Unlock()
		response.Text(request.OriginalUtterance())
		return response
	}
}
