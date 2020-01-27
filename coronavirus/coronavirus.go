package coronavirus

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/azzzak/alice"
	"github.com/go-bongo/bongo"
	"github.com/patrickmn/go-cache"
	"gopkg.in/mgo.v2/bson"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"yandex-dialogs/common"
)

var mongoConnection = common.GetEnv("COMMON_MONGO_CONNECTION", "")
var databaseName = common.GetEnv("COMMON_DATABASE_NAME", "common")
var statusCache = cache.New(5*time.Minute, 10*time.Minute)

var shortPhrases = []string{"Число заразившихся на сегодняшний день достигло %d человек, умерли %d человек.", "На данный момент коронавирусом заразилось %d человек, умерли %d человек."}

var acceptNews = []string{"да", "давай", "можно", "плюс", "ага", "угу", "дэ", "новости", "что в мире"}
var helpWords = []string{"помощь", "что ты може", "что ты умеешь"}
var cancelWords = []string{"отмена", "хватит", "все", "всё", "закончи", "закончить", "выход", "выйди", "выйти"}
var runSkillWords = []string{"Хрен знает, выживший, на кой ляд тебе этот коронавирус сдался, но я в чужие дела не лезу.", "Здравствуй, выживший!", "Поздравляю, вы всё ещё живы! А тем временем", "Добро, выживший!", "Приветствую, выживший!"}
var endSkillWords = []string{"Удачи, выживший!", "Ну бывай, выживший!", "Не хворай, выживший!", "Не болей, выживший!"}
var newsWords = []string{"Хочешь послушать полную сводку?", "Послушаешь подробно?", "Рассказать подробнее?"}

var defaultAnswer = &DayStatus{
	Short:  "Выживший... Сервера пали... Связи больше нет.",
	News:   "Хрен знает на кой ляд тебе эти новости сдались, но я в чужие дела не лезу, хочешь, значит есть зачем... только вот сервера всё равно недоступны.",
	Status: []string{"Скорее всего апокалипсис уже наступил."},
}

type DayStatus struct {
	bongo.DocumentBase `bson:",inline"`
	Short              string   `json:"-"`
	Cases              int      `json:"-,"`
	Death              int      `json:"-,"`
	News               string   `json:"-,"`
	Status             []string `json:"-,"`
}

type CountryInfo struct {
	Region string `json:"region"`
	Cases  string `json:"cases"`
	Death  string `json:"death"`
}

type Coronavirus struct {
	mux        sync.Mutex
	connection *bongo.Connection
	httpClient http.Client
}

func NewCoronavirus() Coronavirus {
	rand.Seed(time.Now().Unix())
	config := &bongo.Config{
		ConnectionString: mongoConnection,
		Database:         databaseName,
	}
	connection, err := bongo.Connect(config)
	if err != nil {
		log.Fatal(err)
	}
	connection.Session.SetPoolLimit(50)
	return Coronavirus{
		connection: connection,
		httpClient: http.Client{Timeout: time.Millisecond * 2000},
	}
}

func (c Coronavirus) GetPath() string {
	return "/api/dialogs/coronavirus"
}

func (c Coronavirus) Health() (result bool, message string) {
	if c.Ping() != nil {
		log.Printf("Ping failed")
		c.Reconnect()
		return false, "DB is not available"
	}
	return true, "OK"
}

func (c Coronavirus) Reconnect() {
	err := c.connection.Connect()
	if err != nil {
		log.Print(err)
	}
	c.connection.Session.SetPoolLimit(50)
}

func (c Coronavirus) Ping() (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("Recovered in f", r)
			err = errors.New("Error ping")
		}
	}()
	err = c.connection.Session.Ping()
	return err
}

func (c Coronavirus) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
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

		currentStatus := c.GetDayStatus()

		if containsIgnoreCase(request.Text(), helpWords) {
			response.Text("Это твой личный гид в хроники коронавируса. Полезная хреновина, которая помогает подготовиться на случай возможной эпидемии. А если и так, то хоть будешь знать, когда консервы покупать, хе-хе-хе... Просто слушай сводку за день и следуй указаниям навыка.")
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), acceptNews) {
			response.Text(currentStatus.News)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), cancelWords) {
			text := endSkillWords[rand.Intn(len(endSkillWords))]
			response.Text(text + " Скажи - закончить, чтобы я отключился.")
			response.Button("Оценить навык", "https://dialogs.yandex.ru/store/skills/d5087c0d-hroniki-koronavirusa", false)
			response.Button("Закончить", "", false)
			return response
		}

		text := runSkillWords[rand.Intn(len(runSkillWords))]
		text += " "
		text += currentStatus.Short
		text += " "
		text += currentStatus.Status[rand.Intn(len(currentStatus.Status))]
		text += " "
		text += newsWords[rand.Intn(len(newsWords))]
		response.Text(text)
		response.Button("Да, давай новости", "", false)
		response.Button("Выйти", "", false)
		return response
	}
}

func (c Coronavirus) GetDayStatus() *DayStatus {
	status := &DayStatus{}
	err := c.connection.Collection("coronavirus").FindOne(bson.M{}, status)
	if err != nil {
		return defaultAnswer
	}

	var countryInfos []CountryInfo
	resp, err := c.httpClient.Get("https://coronavirus.zone/data.json")
	if err != nil {
		return c.buildErrorStatus(status)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return c.buildErrorStatus(status)
	}

	err = json.Unmarshal(bodyBytes, &countryInfos)
	if err != nil {
		return c.buildErrorStatus(status)
	}

	cases := 0
	death := 0
	for _, info := range countryInfos {
		caseVar, _ := strconv.Atoi(info.Cases)
		cases += caseVar

		deathVar, _ := strconv.Atoi(info.Death)
		death += deathVar
	}

	if cases > 0 && death > 0 {
		status.Short = fmt.Sprintf(shortPhrases[rand.Intn(len(shortPhrases))], cases, death)

		if status.Cases != cases || status.Death != death {
			status.Death = death
			status.Cases = cases
			err = c.connection.Collection("coronavirus").Save(status)
			if err != nil {
				log.Print("Error when saving to DB")
			}
		}
		return status
	} else {
		return defaultAnswer
	}
}

func (c Coronavirus) buildErrorStatus(status *DayStatus) *DayStatus {
	if status.Cases > 0 && status.Death > 0 {
		status.Short = fmt.Sprintf(shortPhrases[rand.Intn(len(shortPhrases))], status.Cases, status.Death)
		return defaultAnswer
	} else {
		return defaultAnswer
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
