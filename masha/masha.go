package masha

import (
	"github.com/azzzak/alice"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
	"yandex-dialogs/common"
)

var helloSentences = [...]string{"Привет", "Добрый день", "Здравствуйте"}

type Masha struct {
	mashaUrl string
}

func NewMasha() Masha {
	rand.Seed(time.Now().Unix())
	return Masha{mashaUrl: common.GetEnv("MASHA_URL", "")}
}

func (v Masha) GetPath() string {
	return "/api/dialogs/masha"
}

func (v Masha) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) *alice.Response {

		text := request.Text()
		if request.Session.New == true {
			text = helloSentences[rand.Intn(len(helloSentences))]
		} else if text == "всё" || text == "закончили" || strings.HasPrefix(text, "хватит") || strings.HasPrefix(text, "выключи") {
			response.Response.EndSession = true
		}
		answer := v.getAnswer(request, text)

		response.Text(answer)
		return response
	}
}

func (v Masha) getAnswer(request *alice.Request, text string) string {
	resp, err := http.PostForm(
		v.mashaUrl,
		url.Values{
			"chatId":  {request.Session.UserID},
			"message": {text},
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	bodyString := string(bodyBytes)
	return bodyString
}
