package voice_mail

import (
	"github.com/azzzak/alice"
	"log"
	"sync"
)

type VoiceMail struct {
	states map[string]string
	mux    sync.Mutex
}

func (v VoiceMail) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) *alice.Response {
		v.mux.Lock()
		defer v.mux.Unlock()
		log.Printf("Entities: %+v", request.Request.NLU.Entities)
		response.Text(request.OriginalUtterance())
		return response
	}
}
