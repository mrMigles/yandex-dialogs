package voice_mail

import (
	"fmt"
	"github.com/azzzak/alice"
	"github.com/go-bongo/bongo"
	"gopkg.in/mgo.v2/bson"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
	"yandex-dialogs/common"
)

var acceptWords = []string{"да", "давай", "можно", "плюс", "ага", "угу", "дэ"}
var negativeWords = []string{"нет", "не", "не надо"}
var helpWords = []string{"что ты умеешь", "help", "помог", "помощь", "что делать", "как", "не понятно", "не понял", "не понятно", "что дальше"}
var nextWords = []string{"дальше", "еще", "ещё", "еше", "следующ", "продолж"}
var cancelWords = []string{"отмена", "хватит", "всё", "закончили"}
var newMessageWords = []string{"новое сообщение", "новое письмо", "отправить", "отправь", "письмо"}
var sendWords = []string{"отправить", "отправляй", "запускай"}
var replyWords = []string{"ответить", "ответь", "reply"}
var checkMailWords = []string{"открой почту", "сообщения", "входящие", "проверь почту", "проверить почту", "что там у меня", "есть новые сообщения", "письма", "ящик", "проверь", "проверить"}
var blackListWords = []string{"забань", "добавь в черный список", "черный список", "чёрный список"}
var myNumberWords = []string{"мой номер", "какой номер", "меня номер"}

var mongoConnection = common.GetEnv("MONGO_CONNECTION", "")
var databaseName = common.GetEnv("DATABASE_NAME", "voice-mail")
var encryptKey = common.GetEnv("ENCRYPT_KEY", "")

type User struct {
	bongo.DocumentBase `bson:",inline"`
	Number             int    `json:"-,"`
	Name               string `json:"-,"`
	Id                 string `json:"-,"`
	BlackList          []int  `json:"-,"`
}

type Message struct {
	bongo.DocumentBase `bson:",inline"`
	From               int    `json:"-,"`
	To                 int    `json:"-,"`
	Text               string `json:"-,"`
}

type UserState struct {
	user    *User
	state   string
	context *Message
}

type VoiceMail struct {
	states     map[string]*UserState
	mux        sync.Mutex
	connection *bongo.Connection
}

func NewVoiceMail() VoiceMail {
	rand.Seed(time.Now().Unix())
	config := &bongo.Config{
		ConnectionString: mongoConnection,
		Database:         databaseName,
	}
	connection, err := bongo.Connect(config)

	if err != nil {
		log.Fatal(err)
	}
	return VoiceMail{
		states:     map[string]*UserState{},
		connection: connection,
	}
}

func (v VoiceMail) GetPath() string {
	return "/api/dialogs/voice-mail"
}

func (v VoiceMail) Health() (result bool, message string) {
	if v.connection.Session.Ping() != nil {
		return false, "DB is not available"
	}
	return true, "OK"
}

func (v VoiceMail) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) *alice.Response {

		currentUser := v.findUser(request.Session.UserID)

		// if new user
		if currentUser == nil {
			currentUser = &User{
				Id:        request.Session.UserID,
				Number:    50000 + rand.Intn(99999-50000),
				BlackList: []int{},
			}
			v.states[currentUser.Id] = &UserState{user: currentUser, state: "root"}
			err := v.connection.Collection("users").Save(currentUser)
			if err != nil {
				response.Text("Произошла ошибка, попробуйте ещё раз")
				return response
			}

			response.Text(fmt.Sprintf("Добро пожаловать в голосовую почту! Ваш почтовый номер: %d. Чтобы отправить сообщение, просто скажите - новое сообщение, или - отправить. "+
				"Чтобы проверить почту, скажите - проверь почту. Если появятся вопросы, просто скажите - помощь или спросите меня.", currentUser.Number))
			return response
		}

		if request.Session.New {
			v.states[currentUser.Id] = &UserState{user: currentUser, state: "root"}

			text := fmt.Sprintf("Здравствуйте, %d! ", currentUser.Number)
			count := v.getCountOfMessages(currentUser)

			if count > 0 {
				text += fmt.Sprintf("У вас %d %s. Хотите прослушать?", count, alice.Plural(count, "новое сообщение", "новых сообщения", "новых сообщений"))
				v.states[currentUser.Id].state = "ask_start_listen_mail"
			} else {
				text += "У вас нет новых сообщений."
			}
			response.Text(text)
			return response
		}

		// if there is state
		if currentState, ok := v.states[request.Session.UserID]; ok {

			// for main menu questions
			if currentState.state == "root" {

				// for check mail box phrase
				if containsIgnoreCase(request.Text(), checkMailWords) {
					count := v.getCountOfMessages(currentUser)
					if count > 0 {
						response.Text(fmt.Sprintf("У вас %d %s. Хотите прослушать?", count, alice.Plural(count, "новое сообщение", "новых сообщения", "новых сообщений")))
						currentState.state = "ask_start_listen_mail"
					} else {
						response.Text("У вас нет новых сообщений.")
					}
					return response
				}

				// for send new mail phrase
				if containsIgnoreCase(request.Text(), newMessageWords) {
					currentState.state = "ask_send_number"
					currentState.context = &Message{From: currentUser.Number}
					response.Text("Назовите номер получателя?")
					return response
				}

				// for my number phrase
				if containsIgnoreCase(request.Text(), myNumberWords) {
					response.Text(fmt.Sprintf("Ваш номер: %d", currentUser.Number))
					return response
				}

				// for help phrase
				if containsIgnoreCase(request.Text(), helpWords) {
					response.Text("Помощь помощь Помощь помощь")
					return response
				}

				response.Text("Чтобы отправить сообщение, скажите отправить. Для того, чтобы проверить почту, скажите - проверить почту.")
				return response
			}
			if currentState.state == "ask_start_listen_mail" {
				// for yes phrase
				if containsIgnoreCase(request.Text(), acceptWords) {
					message := v.getFirstMessage(currentUser)
					if message == nil {
						response.Text("У вас нет новых сообщений.")
						currentState.state = "root"
						return response
					}
					text := fmt.Sprintf("Сообщение от номера %d. %s . Конец сообщения. Слушать дальше или ответить?", message.From, message.Text)
					response.Text(text)
					currentState.context = message
					currentState.state = "ask_continue_listen_mail"
					err := v.connection.Collection("messages").DeleteDocument(message)
					if err != nil {
						log.Printf("Error: %v", err)
					}
					return response
				}

				// for no phrase
				if containsIgnoreCase(request.Text(), negativeWords) || containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					response.Text("Окей, хотите что-то ещё?")
					return response
				}

				response.Text("Помощь при прослушивании. Информация про блек лист.")
				return response
			}
			if currentState.state == "ask_continue_listen_mail" {
				// for yes phrase
				if containsIgnoreCase(request.Text(), acceptWords) || containsIgnoreCase(request.Text(), nextWords) {
					message := v.getFirstMessage(currentUser)
					if message == nil {
						response.Text("У вас нет новых сообщений.")
						currentState.state = "root"
						return response
					}
					text := fmt.Sprintf("Сообщение от номера %d. %s . Конец сообщения. Слушать дальше или ответить?", message.From, message.Text)
					response.Text(text)
					currentState.state = "ask_continue_listen_mail"
					currentState.context = message
					err := v.connection.Collection("messages").DeleteDocument(message)
					if err != nil {
						log.Printf("Error: %v", err)
					}
					return response
				}

				// for no phrase
				if containsIgnoreCase(request.Text(), negativeWords) || containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState = nil
					response.Text("Окей, хотите что-то ещё?")
					return response
				}

				// for reply phrase
				if containsIgnoreCase(request.Text(), replyWords) {
					if currentState.context == nil {
						response.Text("Сообщение для ответа не выбрано.")
						currentState.state = "root"
						return response
					}
					toMessage := &Message{To: currentState.context.From, From: currentUser.Number}
					currentState.context = toMessage
					text := fmt.Sprintf("Скажите текст сообщения?")
					response.Text(text)
					currentState.state = "ask_send_text"
					return response
				}

				// for black list phrase
				if containsIgnoreCase(request.Text(), blackListWords) {
					if currentState.context == nil {
						response.Text("Сообщение для блек листа не выбрано.")
						currentState.state = "root"
						return response
					}
					currentUser.BlackList = append(currentUser.BlackList, currentState.context.From)

					text := fmt.Sprintf("Номер %d был добавлен в черный список. Для того, чтобы очистить список, просто скажите - очистить черный список. "+
						"Хотите продолжить прослушивание сообщений?", currentState.context.From)
					response.Text(text)
					currentState.state = "ask_after_black_list"
					return response
				}

				response.Text("Помощь при выбор ответа. Информация про блек лист.")
				return response
			}
			if currentState.state == "ask_after_black_list" {
				// for yes phrase
				if containsIgnoreCase(request.Text(), acceptWords) || containsIgnoreCase(request.Text(), nextWords) {
					message := v.getFirstMessage(currentUser)
					if message == nil {
						response.Text("У вас нет новых сообщений.")
						currentState.state = "root"
						return response
					}
					text := fmt.Sprintf("Сообщение от номера %d. %s . Конец сообщения. Слушать дальше или ответить?", message.From, message.Text)
					response.Text(text)
					currentState.state = "ask_continue_listen_mail"
					err := v.connection.Collection("messages").DeleteDocument(message)
					if err != nil {
						log.Printf("Error: %v", err)
					}
					return response
				}

				currentState.state = "root"
				response.Text("Окей, хотите что-то ещё?")
				return response

			}
			if currentState.state == "ask_send_number" {
				// for no phrase
				if containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState = nil
					response.Text("Окей, хотите что-то ещё?")
					return response
				}

				if currentState.context == nil {
					response.Text("Скажите отправить новое сообщение, для того чтобы отправить")
					currentState.state = "root"
					return response
				}
				var number string
				tokens := request.Tokens()
				for _, num := range tokens {
					number += num
				}
				to, err := strconv.Atoi(number)
				if err != nil {
					response.Text("Вам нужно назвать пятизначный номер получателя, или скажите - отмена, чтобы вернуться в главное меню.")
					return response
				}
				currentState.context.To = to
				text := fmt.Sprintf("Скажите текст сообщения?")
				response.Text(text)
				currentState.state = "ask_send_text"
				return response
			}
			if currentState.state == "ask_send_text" {

				// for help phrase
				if containsIgnoreCase(request.Text(), helpWords) {
					response.Text("Помощь помощь Помощь помощь")
					return response
				}

				// for no phrase
				if containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState = nil
					response.Text("Окей, хотите что-то ещё?")
					return response
				}

				if currentState.context == nil {
					response.Text("Скажите отправить новое сообщение, для того чтобы отправить")
					currentState.state = "root"
					return response
				}

				currentState.context.Text = request.Text()
				currentState.state = "ask_send_confirm"
				response.Text(fmt.Sprintf("Отправить сообщение на номер - %d, c текстом - %s?", currentState.context.To, currentState.context.Text))
				return response

			}
			if currentState.state == "ask_send_confirm" {
				// for yes phrase
				if containsIgnoreCase(request.Text(), acceptWords) || containsIgnoreCase(request.Text(), sendWords) {
					currentState.state = "root"
					err := v.connection.Collection("messages").Save(currentState.context)
					if err != nil {
						response.Text("Произошла ошибка, попробуйте ещё раз")
						return response
					}
					currentState = nil
					response.Text("Сообщение отправлено! Хотите что-то ещё?")
					return response
				}

				// for no phrase
				if containsIgnoreCase(request.Text(), negativeWords) || containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState = nil
					response.Text("Окей, хотите что-то ещё?")
					return response
				}

				response.Text("Что подтвердить отправку сообщения, скажите - да. Скажите - отмена или -нет, чтобы вернуться в главое меню")
				return response
			}

		} else {
			v.states[currentUser.Id] = &UserState{user: currentUser, state: "root"}
			response.Text("Что пожелаете?")
			return response
		}

		response.Text(request.OriginalUtterance())
		return response
	}
}
func (v VoiceMail) getFirstMessage(currentUser *User) *Message {
	message := &Message{}
	err := v.connection.Collection("messages").FindOne(bson.M{"to": currentUser.Number}, message)

	if err != nil {
		log.Printf("Messages for user %d not found", currentUser.Number)
		return nil
	} else {
		log.Printf("Found message %v for user %d", message.GetId(), currentUser.Number)
	}
	return message
}

func (v VoiceMail) getCountOfMessages(currentUser *User) int {
	results := v.connection.Collection("messages").Find(bson.M{"to": currentUser.Number})
	count := 0
	message := &Message{}
	for results.Next(message) {
		count++
	}
	return count
}

func (v VoiceMail) findUser(userId string) *User {
	user := &User{}
	err := v.connection.Collection("users").FindOne(bson.M{"id": userId}, user)

	if err != nil {
		log.Printf("User %s not found", userId)
		return nil
	} else {
		log.Printf("Found user: %+v", user)
	}
	return user
}

func containsIgnoreCase(message string, wordsToCheck []string) bool {
	for _, word := range wordsToCheck {
		if strings.Contains(strings.ToUpper(message), strings.ToUpper(word)) {
			return true
		}
	}
	return false
}
