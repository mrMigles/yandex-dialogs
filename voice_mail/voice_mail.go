package voice_mail

import (
	"errors"
	"fmt"
	"github.com/azzzak/alice"
	"github.com/go-bongo/bongo"
	"gopkg.in/mgo.v2/bson"
	"hash/fnv"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"yandex-dialogs/common"
)

var acceptWords = []string{"да", "давай", "можно", "плюс", "ага", "угу", "дэ"}
var negativeWords = []string{"нет", "не", "не надо"}
var helpWords = []string{"что ты умеешь", "help", "помог", "помощь", "что делать", "как", "не понятно", "не понял", "не понятно", "что дальше"}
var nextWords = []string{"дальше", "еще", "ещё", "еше", "следующ", "продолж"}
var cancelWords = []string{"отмена", "хватит", "все", "всё", "закончи", "закончить", "выход"}
var newMessageWords = []string{"новое сообщение", "новое письмо", "отправить", "отправь", "письмо"}
var sendWords = []string{"отправить", "отправляй", "запускай"}
var replyWords = []string{"ответить", "ответ", "reply"}
var repeatWords = []string{"повтор", "расслышал"}
var checkMailWords = []string{"открой почту", "сообщения", "входящие", "проверь почту", "проверить почту", "что там у меня", "есть новые сообщения", "письма", "ящик", "проверь", "проверить"}
var blackListWords = []string{"забань", "добавь в черный список", "черный список", "чёрный список"}
var myNumberWords = []string{"мой номер", "какой номер", "меня номер"}

var runSkillWords = []string{"говорящая почта", "говорящую почту", "говорящей почты", "запусти навык"}

var mongoConnection = common.GetEnv("MONGO_CONNECTION", "")
var databaseName = common.GetEnv("DATABASE_NAME", "voice-mail")
var encryptKey = common.GetEnv("ENCRYPT_KEY", "")

type User struct {
	bongo.DocumentBase `bson:",inline"`
	Number             int    `json:"-,"`
	Name               string `json:"-,"`
	Id                 string `json:"-,"`
	BlackList          []int  `json:"-,"`
	LastNumber         int    `json:"-,"`
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

func (v VoiceMail) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) *alice.Response {

		currentUser := v.findUser(request.Session.UserID)

		// if new user
		if currentUser == nil {
			if containsIgnoreCase(request.Text(), runSkillWords) {
				response.Text("Запускаюсь")
				return response
			}
			number, err := v.generateNumber(request.Session.UserID)
			if err != nil {
				response.Text("Произошла ошибка, попробуйте в другой раз")
				response.EndSession()
				return response
			}
			currentUser = &User{
				Id:        request.Session.UserID,
				Number:    number,
				BlackList: []int{},
			}
			v.states[currentUser.Id] = &UserState{user: currentUser, state: "root"}
			err = v.connection.Collection("users").Save(currentUser)
			if err != nil {
				response.Text("Произошла ошибка, попробуйте ещё раз")
				return response
			}

			helloMessage := &Message{From: 1000, To: number, Text: "Добро пожаловать в ряды пользователей Говорящей почты! " +
				"Это первое, приветсвенное 'Hello World' сообщение от создателя навыка. " +
				"Вы можете использовать номер 1-0-0-0 для отправки ваших отзывов и предложений по навыку. " +
				"Иногда с этого номера будет приходить важная информация об изменениях в работе навыка. " +
				"Ответьте на данное сообщение, если у вас есть идеи, как можно сделать Говорящую почту лучше. " +
				"Спасибо, что пользуетесь навыком! " +
				"Конец связи."}
			err = v.connection.Collection("messages").Save(helloMessage)
			if err != nil {
				response.Text("Произошла ошибка, попробуйте ещё раз")
				return response
			}

			response.Text(fmt.Sprintf("Добро пожаловать в говорящую почту! Ваш почтовый номер: %s."+
				" Поделитесь этим номером с друзьями, и они смогут присылать вам сообщения."+
				" Вы можете отправить новое сообщение или проверить почту."+
				" Если появятся вопросы, просто скажите - помощь, или задайте вопрос."+
				" С чего начнём?", v.printNumber(currentUser.Number)))
			response.Button("Отправить", "", true)
			response.Button("Проверить почту", "", true)
			response.Button("Помощь", "", true)
			return response
		}

		if request.Session.New {
			v.states[currentUser.Id] = &UserState{user: currentUser, state: "root"}
		}

		if request.Text() == "" {
			text := fmt.Sprintf("Здравствуйте! ")
			count := v.getCountOfMessages(currentUser)

			if count > 0 {
				text += fmt.Sprintf("У вас %s %s. Хотите прослушать?", v.printCount(count), alice.Plural(count, "новое сообщение", "новых сообщения", "новых сообщений"))
				v.states[currentUser.Id].state = "ask_start_listen_mail"
				response.Button("Да", "", true)
				response.Button("Нет", "", true)
			} else {
				text += "У вас нет новых сообщений. Скажите - отправить, чтобы отправить новое сообщение."
				response.Button("Отправить", "", true)
				response.Button("Мой номер", "", true)
				response.Button("Закончить", "", true)
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
						response.Text(fmt.Sprintf("У вас %s %s. Хотите прослушать?", v.printCount(count), alice.Plural(count, "новое сообщение", "новых сообщения", "новых сообщений")))
						currentState.state = "ask_start_listen_mail"
						response.Button("Да", "", true)
						response.Button("Нет", "", true)
					} else {
						response.Text("У вас нет новых сообщений.")
						response.Button("Отправить", "", true)
						response.Button("Закончить", "", true)
					}
					return response
				}

				// for send new mail phrase
				if containsIgnoreCase(request.Text(), newMessageWords) {
					currentState.state = "ask_send_number"
					currentState.context = &Message{From: currentUser.Number}
					response.Text("Назовите номер получателя")
					if currentUser.LastNumber > 0 {
						response.Button(v.printNumber(currentUser.LastNumber), "", true)
					}
					response.Button("1-0-0-0", "", true)
					response.Button("Отмена", "", true)
					return response
				}

				// for my number phrase
				if containsIgnoreCase(request.Text(), myNumberWords) {
					response.Text(fmt.Sprintf("Ваш номер: %s", v.printNumber(currentUser.Number)))
					response.Button("Отправить", "", true)
					response.Button("Закончить", "", true)
					return response
				}

				// for help phrase
				if containsIgnoreCase(request.Text(), helpWords) {
					response.Text("Для того, чтобы отправить сообщение, скажите - отправить. " +
						"Чтобы проверить почту, скажите - проверить почту. " +
						"Чтобы узнать свой номер, скажите - мой номер. " +
						"Чтобы отменить текущую операцию, скажите - отмена. Скажите - закончить, чтобы выйти из навыка.")
					response.Button("Отправить", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Мой номер", "", true)
					response.Button("Закончить", "", true)
					return response
				}

				if strings.EqualFold(request.Text(), "Закончить") {
					response.EndSession()
					response.Text("До свидания!")
					return response
				}

				// for cancel phrase
				if containsIgnoreCase(request.Text(), cancelWords) || containsIgnoreCase(request.Text(), negativeWords) {
					response.Text("Хорошо, заходите ещё! Скажите - закончить, чтобы выйти из навыка.")
					response.Button("Оценить навык", "https://dialogs.yandex.ru/store/skills/eacbce8f-govoryashaya-po", false)
					response.Button("Закончить", "", false)
					currentState = nil
					return response
				}

				response.Text("Чтобы отправить сообщение, скажите отправить. Для того, чтобы проверить почту, скажите - проверить почту.")
				response.Button("Отправить", "", true)
				response.Button("Проверить почту", "", true)
				response.Button("Закончить", "", true)
				return response
			}
			if currentState.state == "ask_start_listen_mail" {
				// for yes phrase
				if containsIgnoreCase(request.Text(), acceptWords) {
					message := v.getFirstMessage(currentUser)
					if message == nil {
						response.Text("У вас нет новых сообщений.")
						response.Button("Отправить", "", true)
						response.Button("Закончить", "", true)
						currentState.state = "root"
						return response
					}
					text := fmt.Sprintf("Сообщение от номера: %s. %s. - . Слушать дальше или ответить?", v.printNumber(message.From), message.Text)
					response.Text(text)
					currentState.context = message
					currentState.state = "ask_continue_listen_mail"
					err := v.connection.Collection("messages").DeleteDocument(message)
					if err != nil {
						log.Printf("Error: %v", err)
					}
					response.Button("Дальше", "", true)
					response.Button("Ответить", "", true)
					return response
				}

				// for no phrase
				if containsIgnoreCase(request.Text(), negativeWords) || containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState.context = nil
					response.Text("Окей, хотите что-то ещё?")
					response.Button("Отправить", "", true)
					response.Button("Нет", "", true)
					return response
				}

				response.Text("Скажите - да, чтобы перейти к прослушиванию сообщений. Или - отмена, чтобы выйти в главное меню.")
				response.Button("Да", "", true)
				response.Button("Отмена", "", true)
				return response
			}
			if currentState.state == "ask_continue_listen_mail" {
				// for yes phrase
				if containsIgnoreCase(request.Text(), acceptWords) || containsIgnoreCase(request.Text(), nextWords) {
					message := v.getFirstMessage(currentUser)
					if message == nil {
						response.Text("У вас нет новых сообщений.")
						currentState.state = "root"
						response.Button("Отправить", "", true)
						response.Button("Закончить", "", true)
						currentState.context = nil
						return response
					}
					text := fmt.Sprintf("Сообщение от номера: %s. %s. - . Слушать дальше или ответить?", v.printNumber(message.From), message.Text)
					response.Text(text)
					currentState.state = "ask_continue_listen_mail"
					currentState.context = message
					err := v.connection.Collection("messages").DeleteDocument(message)
					if err != nil {
						log.Printf("Error: %v", err)
					}
					response.Button("Дальше", "", true)
					response.Button("Ответить", "", true)
					return response
				}

				// for repeat phrase
				if containsIgnoreCase(request.Text(), repeatWords) {
					if currentState.context == nil {
						response.Text("Сообщение для повтора не выбрано.")
						currentState.state = "root"
						return response
					}
					text := fmt.Sprintf("Сообщение от номера: %s. %s. - . Слушать дальше или ответить?", v.printNumber(currentState.context.From), currentState.context.Text)
					response.Text(text)
					response.Button("Дальше", "", true)
					response.Button("Ответить", "", true)
					return response
				}

				// for no phrase
				if containsIgnoreCase(request.Text(), negativeWords) || containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState.context = nil
					response.Text("Окей, хотите что-то ещё?")
					response.Button("Отправить", "", true)
					response.Button("Нет", "", true)
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
					response.Button("Отмена", "", true)
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

					text := fmt.Sprintf("Номер %s был добавлен в черный список. Для того, чтобы очистить список, просто скажите - очистить черный список. "+
						"Хотите продолжить прослушивание сообщений?", v.printNumber(currentState.context.From))
					response.Text(text)
					currentState.state = "ask_after_black_list"
					return response
				}

				response.Text("Вы можете ответить на это сообщение, сказав - ответить, или продолжить слушать сообщения, просто ответив - дальше. Также, вы можете забанить отправителя сообщения и добавить его в черный список, просто сказав - забанить")
				response.Button("Дальше", "", true)
				response.Button("Ответить", "", true)
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
					text := fmt.Sprintf("Сообщение от номера %s. %s. - Слушать дальше или ответить?", v.printNumber(message.From), message.Text)
					response.Text(text)
					currentState.state = "ask_continue_listen_mail"
					currentState.context = message
					err := v.connection.Collection("messages").DeleteDocument(message)
					if err != nil {
						log.Printf("Error: %v", err)
					}
					response.Button("Дальше", "", true)
					response.Button("Ответить", "", true)
					return response
				}

				currentState.state = "root"
				response.Text("Окей, хотите что-то ещё?")
				response.Button("Отправить", "", true)
				response.Button("Нет", "", true)
				return response

			}
			if currentState.state == "ask_send_number" {
				// for cancel phrase
				if equalsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState.context = nil
					response.Text("Окей, хотите что-то ещё?")
					response.Button("Проверить почту", "", true)
					response.Button("Нет", "", true)
					return response
				}

				if currentState.context == nil {
					response.Text("Скажите - отправить, для того чтобы отправить новое сообщение")
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
					response.Text("Вам нужно назвать четырёхзначный номер получателя, или скажите - отмена, чтобы вернуться в главное меню.")
					response.Button("Отмена", "", true)
					return response
				}
				currentState.context.To = to
				text := fmt.Sprintf("Произнесите текст сообщения")
				if to == 1000 {
					text = fmt.Sprintf("Произнесите текст отзыва или предложения")
				}
				response.Text(text)
				currentState.state = "ask_send_text"
				response.Button("Отмена", "", true)
				return response
			}
			if currentState.state == "ask_send_text" {

				// for help phrase
				if equalsIgnoreCase(request.Text(), helpWords) {
					response.Text("Произнесите текст сообщения, или скажите - отмена, чтобы вернуться в главное меню.")
					response.Button("Отмена", "", true)
					return response
				}

				// for no phrase
				if equalsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState.context = nil
					response.Text("Окей, хотите что-то ещё?")
					response.Button("Проверить почту", "", true)
					response.Button("Нет", "", true)
					return response
				}

				if currentState.context == nil {
					response.Text("Скажите - отправить новое сообщение, для того чтобы отправить")
					currentState.state = "root"
					return response
				}

				currentState.context.Text = request.Text()
				currentState.state = "ask_send_confirm"
				response.Text(fmt.Sprintf("Отправляю сообщение - %s - на номер - %s. Всё верно?", currentState.context.Text, v.printNumber(currentState.context.To)))
				response.Button("Да", "", true)
				response.Button("Нет", "", true)
				return response

			}
			if currentState.state == "ask_send_confirm" {
				// for yes phrase
				if containsIgnoreCase(request.Text(), acceptWords) || containsIgnoreCase(request.Text(), sendWords) {
					currentState.state = "root"
					err := v.connection.Collection("messages").Save(currentState.context)
					if err != nil {
						response.Text("Произошла ошибка, попробуйте ещё раз")
						response.Button("Отмена", "", true)
						return response
					}
					currentUser.LastNumber = currentState.context.To
					err = v.connection.Collection("users").Save(currentUser)
					if err != nil {
						response.Text("Произошла ошибка, попробуйте ещё раз")
						response.Button("Отмена", "", true)
						return response
					}
					currentState.context = nil
					response.Text("Сообщение отправлено! Хотите что-то ещё?")
					response.Button("Проверить почту", "", true)
					response.Button("Нет", "", true)
					return response
				}

				// for no phrase
				if containsIgnoreCase(request.Text(), negativeWords) || containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState.context = nil
					response.Text("Окей, хотите что-то ещё?")
					response.Button("Отправить новое", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Закончить", "", true)
					return response
				}

				response.Text("Чтобы подтвердить отправку сообщения, скажите - да. Либо скажите - отмена, чтобы вернуться в главое меню")
				response.Button("Да", "", true)
				response.Button("Отмена", "", true)
				return response
			}

		} else {
			v.states[currentUser.Id] = &UserState{user: currentUser, state: "root"}
			response.Text("Что пожелаете?")
			response.Button("Отправить новое сообщение", "", true)
			response.Button("Проверить почту", "", true)
			response.Button("Помощь", "", true)
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

func equalsIgnoreCase(message string, wordsToCheck []string) bool {
	for _, word := range wordsToCheck {
		if strings.EqualFold(message, word) {
			return true
		}
	}
	return false
}

func (v VoiceMail) generateNumber(userId string) (int, error) {
	v.mux.Lock()

	rand.Seed(int64(hash(userId)))
	number := 1000 + rand.Intn(9999-1000)
	defer v.mux.Unlock()
	for i := 0; i < 5; i++ {
		user := &User{}
		err := v.connection.Collection("users").FindOne(bson.M{"number": number}, user)
		if err != nil {
			if _, ok := err.(*bongo.DocumentNotFoundError); ok {

				return number, nil
			} else {
				log.Print("real error " + err.Error())
				return 0, err
			}
		}
		number++
	}
	return 0, errors.New("error when generating unique id")
}

func (v VoiceMail) printNumber(number int) string {
	strNumber := strings.Split(strconv.Itoa(number), "")
	return fmt.Sprintf("%s", strings.Join(strNumber, "-"))
}

func (v VoiceMail) printCount(number int) string {
	countStr := strconv.Itoa(number)
	if number == 1 {
		countStr = "одно"
	}
	return countStr
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
