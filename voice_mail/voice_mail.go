package voice_mail

import (
	"github.com/azzzak/alice"
	"sync"
)

type VoiceMail struct {
	states map[string]string
	mux    sync.Mutex
}

func (v VoiceMail) GetPath() string {
	return "/api/dialogs/voice-mail"
}

func (v VoiceMail) Health() (result bool, message string) {
	return true, "OK"
}

func (v VoiceMail) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) *alice.Response {
		response.Text(request.OriginalUtterance())
		return response
	}
}
