package masha

import (
	"fmt"
	"github.com/azzzak/alice"
	"github.com/go-bongo/bongo"
	"github.com/robfig/cron/v3"
	"gopkg.in/mgo.v2/bson"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"yandex-dialogs/common"
	"yandex-dialogs/voice_mail"
)

var helloSentences = [...]string{"Привет", "Добрый день", "Здравствуйте"}

var bySentences = [...]string{"Хорошо поболтали! Запиши мой номер - 8-8-0-0, буду ждать твоего письма.", "Хорошего дня!", "Отличный разговор! Напиши мне, 8-8-0-0", "Окей, буду ждать тут", "Хорошо поболтали, но учти, в следующий раз я могу измениться!", "Спасибо за разговор! Можешь писать мне в говорящую почту, на номер 8-8-0-0"}

var errorSentences = [...]string{"Даже не знаю, спроси что нибудь ещё", "Что-то не могу сообразить, давай поменяем тему", "Не могу сообразить, спроси ещё что нибудь", "Даже не знаю, спроси по другому"}

var exitWords = []string{"отмена", "хватит", "выйти", "закончи", "закрыть", "выход"}

var mongoConnection = common.GetEnv("MONGO_CONNECTION", "")
var databaseName = common.GetEnv("DATABASE_NAME", "voice-mail")

type Masha struct {
	mashaUrl   string
	httpClient http.Client
	connection *bongo.Connection
}

func NewMasha() Masha {
	rand.Seed(time.Now().Unix())

	config := &bongo.Config{
		ConnectionString: mongoConnection,
		Database:         databaseName,
	}
	connection, err := bongo.Connect(config)
	if err != nil {
		log.Print(err)
	}

	masha := Masha{
		mashaUrl:   common.GetEnv("MASHA_URL", ""),
		httpClient: http.Client{Timeout: time.Millisecond * 2700},
		connection: connection,
	}
	c := cron.New()
	_, err = c.AddFunc("*/5 * * * *", masha.mail())
	if err != nil {
		log.Printf("Error running cron for Masha mail: %+v", err)
	} else {
		c.Start()
	}
	return masha
}

func (v Masha) GetPath() string {
	return "/api/dialogs/masha"
}

func (v Masha) Health() (result bool, message string) {
	if _, err := v.getAnswer("health", "Привет"); err != nil {
		return false, fmt.Sprintf("Exception occurred when getting message from API: %v", err)
	}
	if v.connection.Session.Ping() != nil {
		log.Printf("Ping failed")
		v.connection.Session.Close()
		config := &bongo.Config{
			ConnectionString: mongoConnection,
			Database:         databaseName,
		}
		v.connection, _ = bongo.Connect(config)
		return false, "DB is not available"
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
		return errorSentences[rand.Intn(len(errorSentences))], err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return errorSentences[rand.Intn(len(errorSentences))], err
	}
	bodyString := string(bodyBytes)
	return bodyString, nil
}

func (m Masha) mail() func() {
	return func() {
		log.Print("Run cron")
		results := m.connection.Collection("messages").Find(bson.M{"to": 8800})
		message := &voice_mail.Message{}
		for results.Next(message) {
			log.Printf("Cron message from %d", message.From)
			question := message.Text
			answer, err := m.getAnswer(strconv.Itoa(message.From), question)
			if err != nil {
				log.Print(err)
				continue
			}
			answerMessage := &voice_mail.Message{To: message.From, From: 8800, Text: answer}
			err = m.connection.Collection("messages").Save(answerMessage)
			if err != nil {
				log.Print(err)
				continue
			}
			err = m.connection.Collection("messages").DeleteDocument(message)
			if err != nil {
				log.Printf("Error: %v", err)
			}
		}
	}
}

func containsIgnoreCase(message string, wordsToCheck []string) bool {
	for _, word := range wordsToCheck {
		if strings.Contains(strings.ToUpper(message), strings.ToUpper(word)) {
			return true
		}
	}
	return false
}
