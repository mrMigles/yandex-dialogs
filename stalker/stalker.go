package stalker

import (
	"errors"
	"github.com/azzzak/alice"
	"github.com/go-bongo/bongo"
	"gopkg.in/mgo.v2/bson"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
	"yandex-dialogs/common"
)

var mongoConnection = common.GetEnv("COMMON_MONGO_CONNECTION", "")
var databaseName = common.GetEnv("COMMON_DATABASE_NAME", "common")

var helpWords = []string{"помощь", "что ты може", "что ты умеешь"}
var laughWords = []string{"ха ха", "аха", "хах"}
var notFunnyWords = []string{"не смешно"}

type Stalker struct {
	httpClient http.Client
	connection *bongo.Connection
	jokes      []Joke
	context    map[string]string
}

type Joke struct {
	bongo.DocumentBase `bson:",inline"`
	Id                 string   `json:"id"`
	Type               string   `json:"type"`
	Title              string   `json:"title"`
	Tags               []string `json:"tags"`
	Likes              []string `json:"likes"`
	Dislikes           []string `json:"dislikes"`
}

type User struct {
	bongo.DocumentBase `bson:",inline"`
	Id                 string   `json:"id"`
	Jokes              []string `json:"jokes"`
}

func NewStalker() Stalker {
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
	stalker := Stalker{
		httpClient: http.Client{Timeout: time.Millisecond * 20000},
		connection: connection,
		context:    map[string]string{},
	}
	stalker.initJokes()
	return stalker
}

func (c Stalker) GetPath() string {
	return "/api/dialogs/stalker"
}

func (c Stalker) Health() (result bool, message string) {
	if c.Ping() != nil {
		log.Printf("Ping failed")
		c.Reconnect()
		return false, "DB is not available"
	}
	return true, "OK"
}

func (c Stalker) Reconnect() {
	err := c.connection.Connect()
	if err != nil {
		log.Print(err)
	}
	c.connection.Session.SetPoolLimit(50)
}

func (c Stalker) Ping() (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("Recovered in f", r)
			err = errors.New("Error ping")
		}
	}()
	err = c.connection.Session.Ping()
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

		isNew := false
		user := c.getUser(request.UserID())
		if user == nil {
			user = &User{
				Id: request.UserID(),
			}
			isNew = true
		}

		if containsIgnoreCase(request.Text(), helpWords) {
			response.Text("Рассказываю анекдоты из любимой многими игры. Просто попроси про что рассказать анекдот, и расскажу." +
				"Для того, чтобы оценить андектод - нужно просто посмеяться в ответ, если анекдот не понравился - я думаю, вы знаете что делать.")
			return response
		}

		if containsIgnoreCase(request.Text(), laughWords) {
			response.Text("Уважаю. Слушаем дальше?")
			joke := c.getJokeById(c.context[user.Id])
			if joke != nil {
				if !containsIgnoreCase(user.Id, joke.Likes) {
					joke.Likes = append(joke.Likes, user.Id)
					c.saveJoke(joke)
				}
			}
			response.Button("Да", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), notFunnyWords) {
			response.Text("Ну вот. Слушаем дальше?")
			joke := c.getJokeById(c.context[user.Id])
			if joke != nil {
				if !containsIgnoreCase(user.Id, joke.Dislikes) {
					joke.Likes = append(joke.Likes, user.Id)
					c.saveJoke(joke)
				}
			}
			response.Button("Да", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if len(request.Text()) > 0 {
			num := rand.Intn(len(c.jokes))
			joke := c.jokes[num]
			c.context[user.Id] = joke.Id
			if !containsIgnoreCase(joke.Id, user.Jokes) {
				user.Jokes = append(user.Jokes, joke.Id)
				c.saveUser(user)
			}

			response.CustomSound("be003f01-c4dd-4cf8-96ed-876431d53a49", joke.Id)
			if isNew {
				response.TTS("Чтобы оценить анекдот вы можете посмеяться в ответ, чтобы прослушать следующий - скажите ещё или дальше, скажите повтори - чтобы прсолушать анекдот ещё раз.")
			}
			response.Button("Ахаха", "", true)
			response.Button("Так не смешно же", "", true)
			response.Button("Ещё", "", true)
			response.Button("Повтори", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		response.Text("Здравствуй, Сталкер! Хочешь анекдот?")
		response.Button("Да", "", true)
		response.Button("Выйти", "", true)
		return response
	}
}

func (c *Stalker) initJokes() {
	resultSet := c.connection.Collection("jokes").Find(bson.M{})
	var jokes []Joke
	joke := &Joke{}
	for resultSet.Next(joke) {
		jokes = append(jokes, *joke)
	}
	if len(jokes) > 0 {
		c.jokes = jokes
	}
}

func (c Stalker) getUser(id string) *User {
	user := &User{}
	err := c.connection.Collection("stalkers").FindOne(bson.M{"id": id}, user)
	if err != nil {
		return nil
	}
	return user
}

func (c Stalker) saveUser(user *User) {
	err := c.connection.Collection("stalkers").Save(user)
	if err != nil {
		log.Print("Error when saving to DB")
	}
}

func (c Stalker) saveJoke(joke *Joke) {
	err := c.connection.Collection("jokes").Save(joke)
	if err != nil {
		log.Print("Error when saving to DB")
	}
}

func (c Stalker) getJokeById(id string) *Joke {
	for _, joke := range c.jokes {
		if joke.Id == id {
			return &joke
		}
	}
	return nil
}

func containsIgnoreCase(message string, wordsToCheck []string) bool {
	for _, word := range wordsToCheck {
		if strings.Contains(strings.ToUpper(message), strings.ToUpper(word)) {
			return true
		}
	}
	return false
}
