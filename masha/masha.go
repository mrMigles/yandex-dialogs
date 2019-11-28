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
		httpClient: http.Client{Timeout: time.Millisecond * 2700},
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
			response.Text(fmt.Sprintf("Внимание, диалог может содержать взрослый и непристойный контент, если Вам нет восемнадцати лет, пожалуйста, закройте навык. - %s! Давай поболтаем?", answer))
			return response
		} else if strings.EqualFold(text, "всё") || strings.EqualFold(text, "все") || strings.Contains(text, "хватит") || strings.Contains(text, "закончи") || strings.Contains(text, "выключи") || strings.Contains(text, "выход") {
			response.Response.EndSession = true
		} else if strings.EqualFold(text, "помощь") || strings.EqualFold(text, "что ты умеешь") || strings.Contains(text, "ты умеешь") {
			response.Text("Меня зовут Маша. Я интерактивный бот собеседеник, обучаюсь на разговорах с людьми и каждый день должна становиться умнее. Но практика показывает, что я только деградирую... И вообще, создатель говорит, что отключит меня от сети через пару дней... эх. Просто спроси меня что нибудь, и давай поболтаем. Если устанешь от меня, просто скажи - всё или - хватит болтать.")
			response.Button("Подбодрить Машу", "https://dialogs.yandex.ru/store/skills/67b197f0-nedetskie-razgovory", true)
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

func containsIgnoreCase(message string, wordsToCheck []string) bool {
	for _, word := range wordsToCheck {
		if strings.Contains(strings.ToUpper(message), strings.ToUpper(word)) {
			return true
		}
	}
	return false
}
