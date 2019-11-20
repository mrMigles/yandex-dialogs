package masha

import (
	"fmt"
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
	mashaUrl   string
	httpClient http.Client
}

func NewMasha() Masha {
	rand.Seed(time.Now().Unix())
	return Masha{
		mashaUrl:   common.GetEnv("MASHA_URL", ""),
		httpClient: http.Client{Timeout: time.Millisecond * 2800},
	}
}

func (v Masha) GetPath() string {
	return "/api/dialogs/masha"
}

func (v Masha) Health() (result bool, message string) {
	if _, err := v.getAnswer("health", "Привет"); err != nil {
		return false, fmt.Sprintf("Exception occurred when getting message from API: %v", err)
	}
	return true, "OK"
}

func (v Masha) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) *alice.Response {

		text := request.Text()
		if request.Session.New == true {
			answer := helloSentences[rand.Intn(len(helloSentences))]
			response.Text(fmt.Sprintf("%s! Давай поболтаем?", answer))
			return response
		} else if text == "всё" || text == "закончили" || strings.HasPrefix(text, "хватит") || strings.HasPrefix(text, "выключи") {
			response.Response.EndSession = true
		} else if strings.EqualFold(text, "помощь") || strings.EqualFold(text, "что ты умеешь") || strings.Contains(text, "ты умеешь") {
			response.Text("Меня зовут Маша. Я интерактивный бот собеседеник. Просто спроси меня что нибудь, и давай поболтаем. Если устанешь от меня, просто скажи - всё или - хватит болтать.")
			return response
		}
		answer, _ := v.getAnswer(request.Session.UserID, text)

		response.Text(answer)
		return response
	}
}

func (v Masha) getAnswer(userID string, text string) (string, error) {
	resp, err := v.httpClient.PostForm(
		v.mashaUrl,
		url.Values{
			"chatId":  {userID},
			"message": {text},
		},
	)
	if err != nil {
		log.Print(err)
		return "Даже не знаю, спроси что нибудь ещё", err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return "Даже не знаю, спроси что нибудь ещё", err
	}
	bodyString := string(bodyBytes)
	return bodyString, nil
}
