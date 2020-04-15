package stalker

import (
	"errors"
	"github.com/azzzak/alice"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type Stalker struct {
	httpClient http.Client
}

func NewStalker() Stalker {
	rand.Seed(time.Now().Unix())
	stalker := Stalker{
		httpClient: http.Client{Timeout: time.Millisecond * 20000},
	}
	return stalker
}

func (c Stalker) GetPath() string {
	return "/api/dialogs/stalker"
}

func (c Stalker) Health() (result bool, message string) {
	return true, "OK"
}

func (c Stalker) Reconnect() {

}

func (c Stalker) Ping() (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("Recovered in f", r)
			err = errors.New("Error ping")
		}
	}()
	return err
}

func (c Stalker) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) (resp *alice.Response) {
		defer func() {
			if r := recover(); r != nil {
				log.Print("Recovered in f: ", r)
				response.Text("Произошла ошибка, попробуйте в другой раз")
				response.Button("Закончить", "", true)
				resp = response
			}
		}()
		c.Health()

		response.Text("Так не смешно же.")
		response.CustomSound("be003f01-c4dd-4cf8-96ed-876431d53a49", "15ea89d3-c799-4f0e-b3c0-5bda75ae9448")
		response.Button("Выйти", "", true)
		return response
	}
}
