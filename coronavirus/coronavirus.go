package coronavirus

import (
	"errors"
	"github.com/azzzak/alice"
	"github.com/go-bongo/bongo"
	"github.com/patrickmn/go-cache"
	"gopkg.in/mgo.v2/bson"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
	"yandex-dialogs/common"
)

var mongoConnection = common.GetEnv("MONGO_CONNECTION", "")
var databaseName = common.GetEnv("COMMON_DATABASE_NAME", "common")
var statusCache = cache.New(5*time.Minute, 10*time.Minute)

var acceptNews = []string{"да", "давай", "можно", "плюс", "ага", "угу", "дэ", "новости", "что в мире", "коронавирус"}
var cancelWords = []string{"отмена", "хватит", "все", "всё", "закончи", "закончить", "выход", "выйди", "выйти"}
var runSkillWords = []string{"Хрен знает, выживший, на кой ляд тебе этот коронавирус сдался, но я в чужие дела не лезу.", "Здорово, выживший!", "Поздравляю, вы всё ещё живы! А тем временем", "Добро, выживший!", "Приветствую, выживший!"}
var endSkillWords = []string{"Удачи, выживший!", "Ну бывай, выживший!", "Не хворай, выживший!", "Не болей, выживший!"}
var newsWords = []string{"Хочешь послушать полную сводку?", "Послушаешь подробно?", "Рассказать подробнее?"}

type DayStatus struct {
	bongo.DocumentBase `bson:",inline"`
	Short              string   `json:"-,"`
	Full               string   `json:"-,"`
	Status             []string `json:"-,"`
}

type Coronavirus struct {
	mux        sync.Mutex
	connection *bongo.Connection
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

		if containsIgnoreCase(request.Text(), acceptNews) {
			response.Text(currentStatus.Full)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), cancelWords) {
			text := endSkillWords[rand.Intn(len(endSkillWords))]
			response.Text(text + " Скажи - закончить, чтобы я заткнулся.")
			response.Button("Оценить навык", "https://dialogs.yandex.ru/store/skills", false)
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
		response.Button("Новости", "", false)
		response.Button("Выйти", "", false)
		return response
	}
}

func (c Coronavirus) GetDayStatus() *DayStatus {
	status := &DayStatus{}
	results := c.connection.Collection("coronavirus").FindOne(bson.M{}, status)
	if results != nil {
		return &DayStatus{
			Short:  "Выживший... Сервера пали... Связи больше нет.",
			Full:   "Хрен знает на кой ляд тебе эти новости сдались, но я в чужие дела не лезу, хочешь, значит есть зачем... только вот сервера всё равно недоступны.",
			Status: []string{"Скорее всего апокалипсис уже наступил."},
		}
	}
	return status
}

func containsIgnoreCase(message string, wordsToCheck []string) bool {
	for _, word := range wordsToCheck {
		if strings.Contains(strings.ToUpper(message), strings.ToUpper(word)) {
			return true
		}
	}
	return false
}

func equalsIgnoreCase(message string, wordsToCheck []string) bool {
	for _, word := range wordsToCheck {
		if strings.EqualFold(message, word) {
			return true
		}
	}
	return false
}
