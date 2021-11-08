package masha

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/azzzak/alice"
	"github.com/gorilla/mux"
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
var helloAnswers = [...]string{"Привет, как дела?", "Добрый день", "Здравствуйте", "Что нового?", "как дела?", "Ну давай поболтаем"}

var bySentences = [...]string{"Хорошо поболтали! Запиши мой номер - 8-8-0-0, буду ждать твоего письма.", "Хорошего дня!", "Отличный разговор! Напиши мне, 8-8-0-0", "Окей, буду ждать тут", "Хорошо поболтали, но учти, в следующий раз я могу измениться!", "Спасибо за разговор! Можешь писать мне в говорящую почту, на номер 8-8-0-0"}

var errorSentences = [...]string{"Даже не знаю, спроси что нибудь ещё", "Что-то не могу сообразить, давай поменяем тему", "Не могу сообразить, спроси ещё что нибудь", "Даже не знаю, спроси по другому"}

var failSentences = [...]string{"Что-то мне не хорошо, попробуй зайти попозже", "Что-то не могу нормально соображать, давай притормозим общение на пару часиков", "Я плохо себя чувствую, напиши мне позднее"}

var exitWords = []string{"отмена", "хватит", "выйти", "закончи", "закрыть", "выход"}

var mongoConnection = common.GetEnv("MONGO_CONNECTION", "")
var databaseName = common.GetEnv("DATABASE_NAME", "voice-mail")

var stupidMode = common.GetEnv("MASHA_STUPID_MODE", "false")
var stupidUrl = common.GetEnv("MASHA_STUPID_URL", "")

type Masha struct {
	mashaUrl   string
	httpClient http.Client
}

func (v Masha) ApiHandlers(router *mux.Router) {
	// no implementation here
}

func NewMasha(timeout time.Duration) Masha {
	rand.Seed(time.Now().Unix())

	masha := Masha{
		mashaUrl:   common.GetEnv("MASHA_URL", ""),
		httpClient: http.Client{Timeout: time.Millisecond * timeout},
	}
	return masha
}

func (v Masha) GetPath() string {
	return "/api/dialogs/masha"
}

func (v Masha) Health() (result bool, message string) {
	if _, err := v.GetAnswer("health", "Привет"); err != nil {
		return false, fmt.Sprintf("Exception occurred when getting message from API: %v", err)
	}
	return true, "OK"
}

func (v Masha) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) *alice.Response {

		text := request.Text()
		if request.Session.New == true {
			answer := helloSentences[rand.Intn(len(helloSentences))]
			quest := helloAnswers[rand.Intn(len(helloAnswers))]
			if stupidMode == "true" {
				response.Text(fmt.Sprintf("Привет, друг! Со мной случилась беда: я не могу вспомнить всё, чему обучалась на протяжении этих лет. Прошу, не обижайся на меня, если я буду тупить или отвечать как двухлетний ребенок, я постараюсь вернуть свою память... \n- %s! А пока, Давай поболтаем?", answer))
			} else {
				response.Text(fmt.Sprintf("Внимание, диалог может содержать взрослый и непристойный контент, если Вам нет восемнадцати лет, пожалуйста, закройте навык!. \n- %s! Давай поболтаем?", answer))
			}
			response.Button("Оценить или поддержать Машу", "https://dialogs.yandex.ru/store/skills/67b197f0-nedetskie-razgovory", false)
			response.Button("Написать Маше на почту", "https://dialogs.yandex.ru/store/skills/eacbce8f-govoryashaya-po", false)
			response.Button(quest, "", true)
			return response
		} else if strings.EqualFold(text, "всё") || strings.EqualFold(text, "все") || containsIgnoreCase(text, exitWords) {
			answer := bySentences[rand.Intn(len(bySentences))]
			response.Text(answer)
			if strings.Contains(answer, "8-8-0-0") {
				response.Button("Написать Маше на почту", "https://dialogs.yandex.ru/store/skills/eacbce8f-govoryashaya-po", false)
			} else {
				response.Button("Оценить Машу", "https://dialogs.yandex.ru/store/skills/67b197f0-nedetskie-razgovory", false)
			}
			response.Button("Закончить", "", true)
			response.Response.EndSession = true
			return response
		} else if strings.EqualFold(text, "помощь") || strings.EqualFold(text, "что ты умеешь") || strings.Contains(text, "ты умеешь") || strings.Contains(text, "ты можешь") {
			response.Text("Меня зовут Маша. Я интерактивный бот собеседеник, обучаюсь на разговорах с людьми и каждый день должна становиться умнее. Но практика показывает, что я только деградирую... Просто спроси меня что нибудь, и давай поболтаем. Если устанешь от меня, просто скажи - всё или - хватит болтать. Кстати, мой номер в навыке Говорящая Почта - 8-8-0-0, готова общаться с Вами и там.")
			response.Button("Подбодрить Машу", "https://dialogs.yandex.ru/store/skills/67b197f0-nedetskie-razgovory", false)
			response.Button("Узнать про коронавирус", "https://dialogs.yandex.ru/store/skills/d5087c0d-hroniki-koronavirusa", false)
			return response
		}
		if stupidMode == "true" {
			answer, _ := v.GetStupidAnswer(request.Session.SessionID, text)
			response.Text(answer)
		} else {
			answer, _ := v.GetAnswer(request.Session.UserID, text)
			response.Text(answer)
		}
		return response
	}
}

func (v Masha) GetAnswer(userID string, text string) (string, error) {
	resp, err := v.httpClient.PostForm(
		v.mashaUrl,
		url.Values{
			"chatId":  {userID},
			"message": {text},
		},
	)
	if err != nil {
		log.Print(err)
		return errorSentences[rand.Intn(len(errorSentences))], err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return errorSentences[rand.Intn(len(errorSentences))], err
	}
	bodyString := string(bodyBytes)
	if bodyString == "" {
		log.Print("fail, empty message")
		return failSentences[rand.Intn(len(failSentences))], nil
	}
	return bodyString, nil
}

func (v Masha) GetStupidAnswer(userID string, text string) (string, error) {
	body := map[string]interface{}{}

	body["uid"] = userID
	body["bot"] = "main"
	body["text"] = text

	content, err := json.Marshal(body)
	if err != nil {
		log.Print(err)
		return errorSentences[rand.Intn(len(errorSentences))], err
	}

	resp, err := v.httpClient.Post(
		stupidUrl,
		"application/json",
		bytes.NewBuffer(content),
	)

	if err != nil {
		log.Print(err)
		return errorSentences[rand.Intn(len(errorSentences))], err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return errorSentences[rand.Intn(len(errorSentences))], err
	}
	err = json.Unmarshal(bodyBytes, &body)
	if err != nil {
		log.Print(err)
		return errorSentences[rand.Intn(len(errorSentences))], err
	}

	bodyString := body["text"]
	if bodyString == "" {
		log.Print("fail, empty message")
		return failSentences[rand.Intn(len(failSentences))], nil
	}
	return bodyString.(string), nil
}

func containsIgnoreCase(message string, wordsToCheck []string) bool {
	for _, word := range wordsToCheck {
		if strings.Contains(strings.ToUpper(message), strings.ToUpper(word)) {
			return true
		}
	}
	return false
}
