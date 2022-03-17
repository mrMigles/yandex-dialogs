package voice_mail

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/azzzak/alice"
	"github.com/go-bongo/bongo"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/robfig/cron/v3"
	"hash/fnv"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"yandex-dialogs/common"
)

var acceptWords = []string{"да", "давай", "можно", "плюс", "ага", "угу", "дэ"}
var negativeWords = []string{"нет", "не", "не надо"}
var helpWords = []string{"что ты умеешь", "help", "помог", "помощь", "что делать", "как", "не понятно", "не понял", "не понятно", "что дальше"}
var nextWords = []string{"дальше", "еще", "ещё", "еше", "следующ", "продолж"}
var cancelWords = []string{"отмена", "хватит", "все", "всё", "закончи", "закончить", "выход", "выйди", "выйти"}
var newMessageWords = []string{"новое сообщение", "новое письмо", "отправить", "отправь", "письмо"}
var sendWords = []string{"отправить", "отправляй", "запускай"}
var phoneBookWord = []string{"книг", "записн", "книжк"}
var addPhoneBookWord = []string{"добавить", "запомни", "запиши", "добавь"}
var replyWords = []string{"ответить", "ответ", "reply"}
var repeatWords = []string{"повтор", "расслышал"}
var checkMailWords = []string{"открой почту", "сообщения", "входящие", "проверь почту", "проверить почту", "что там у меня", "есть новые сообщения", "письма", "ящик", "проверь", "проверить"}
var blackListWords = []string{"забань", "добавь в черный список", "черный список", "чёрный список"}
var clearBlackListWords = []string{"очистить черный список", "очисти черный список", "очисть черный список", "очистить чёрный список"}
var myNumberWords = []string{"мой номер", "какой номер", "меня номер"}
var myTokenWords = []string{"токен", "секрет", "пароль", "токинг", "такен"}
var reviewWords = []string{"отзыв", "предложение", "оценк"}
var datingWords = []string{"знаком", "случайн", "рандом", "наугад"}

var runSkillWords = []string{"говорящая почта", "говорящую почту", "говорящей почты", "запусти навык"}

type User struct {
	bongo.DocumentBase `bson:",inline"`
	Number             int            `json:"-,"`
	Name               string         `json:"-,"`
	Id                 string         `json:"-,"`
	BlackList          []int          `json:"-,"`
	LastNumber         int            `json:"-,"`
	PreLastNumber      int            `json:"-,"`
	DateFree           bool           `json:"-,"`
	Reviewed           bool           `json:"-,"`
	PhoneBook          map[string]int `json:"-,"`
}

type Message struct {
	bongo.DocumentBase `bson:",inline"`
	From               int    `json:"from,"`
	To                 int    `json:"to,"`
	Text               string `json:"text,"`
}

type UserState struct {
	user    *User
	state   string
	context *Message
}

type MailBot interface {
	CheckMails()
	GetCron() string
}

type VoiceMail struct {
	states      map[string]*UserState
	mux         sync.Mutex
	mailService *MailService
}

func NewVoiceMail() VoiceMail {
	mailService := NewMailService()
	initBots(mailService)
	return VoiceMail{
		states:      map[string]*UserState{},
		mailService: mailService,
	}
}

func initBots(service *MailService) {
	mashaBot := NewMashaBot(service)
	datingBot := NewDatingBot(service)
	c := cron.New()

	c.AddFunc(mashaBot.GetCron(), func() {
		mashaBot.CheckMails()
	})
	c.AddFunc(datingBot.GetCron(), func() {
		datingBot.CheckMails()
	})
	c.Start()

}

func (v VoiceMail) GetPath() string {
	return "/api/dialogs/voice-mail"
}

func (v VoiceMail) ApiHandlers(r *mux.Router) {
	handler := common.Handler()
	r.Handle("/api/v1/dialogs/voice-mail/receive",
		handlers.LoggingHandler(
			os.Stdout,
			handler(v.handleReceiveRequest())),
	).Methods("GET")

	r.Handle("/api/v1/dialogs/voice-mail/send",
		handlers.LoggingHandler(
			os.Stdout,
			handler(v.handleSendRequest())),
	).Methods("POST")
}

func (v VoiceMail) handleReceiveRequest() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.WriteHeader(401)
			w.Write([]byte("Unauthorized access"))
			return
		}

		userId := common.BearerAuthHeader(authHeader)
		if userId == "" {
			w.WriteHeader(403)
			w.Write([]byte("Incorrect authorization header"))
			return
		}

		user, err := v.mailService.findUser(userId)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Something went wrong"))
			return
		}
		if user == nil {
			w.WriteHeader(403)
			w.Write([]byte("Cannot authorize receiving messages for specified user token"))
			return
		}

		message := v.mailService.ReadMessage(user)

		if message == nil {
			w.WriteHeader(404)
			w.Write([]byte("{}"))
			return
		}

		w.WriteHeader(200)
		response, err := json.Marshal(message)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Something went wrong"))
			return
		}
		w.Write(response)
	}
}

func (v VoiceMail) handleSendRequest() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.WriteHeader(401)
			w.Write([]byte("Unauthorized access"))
			return
		}

		userId := common.BearerAuthHeader(authHeader)
		if userId == "" {
			w.WriteHeader(403)
			w.Write([]byte("Incorrect authorization header"))
			return
		}

		user, err := v.mailService.findUser(userId)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Something went wrong"))
			return
		}
		if user == nil {
			w.WriteHeader(403)
			w.Write([]byte("Cannot authorize receiving messages for specified user token"))
			return
		}

		numberVar := r.PostFormValue("number")
		if numberVar == "" {
			w.WriteHeader(400)
			w.Write([]byte("Incorrect number format"))
			return
		}
		number, err := strconv.Atoi(numberVar)
		if err != nil || number < 1000 || number >= 100000 {
			w.WriteHeader(400)
			w.Write([]byte("Incorrect number format"))
			return
		}

		text := r.PostFormValue("text")
		if text == "" {
			w.WriteHeader(400)
			w.Write([]byte("text is empty"))
			return
		}

		message := &Message{From: user.Number, To: number, Text: text}
		err = v.mailService.SendMessage(message)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Something went wrong"))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("Sent"))
	}
}

func (v VoiceMail) Health() (result bool, message string) {
	if v.mailService.Ping() != nil {
		log.Printf("Ping failed")
		v.mailService.Reconnect()
		return false, "DB is not available"
	}
	return true, "OK"
}

func (v VoiceMail) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) (resp *alice.Response) {
		defer func() {
			if r := recover(); r != nil {
				log.Print("Recovered in f: ", r)
				response.Text("Произошла ошибка, попробуйте в другой раз")
				response.Button("Закончить", "", true)
				resp = response
			}
		}()
		v.Health()
		currentUser, err := v.mailService.findUser(request.Session.UserID)
		if err != nil {
			response.Text("Произошла ошибка, попробуйте в другой раз")
			response.Button("Закончить", "", true)
			return response
		}

		// if new user
		if currentUser == nil {
			if containsIgnoreCase(request.Text(), runSkillWords) {
				response.Text("Запускаюсь")
				return response
			}
			number, err := v.generateNumber(request.Session.UserID)
			if err != nil {
				response.Text("Произошла ошибка, попробуйте в другой раз")
				response.Button("Закончить", "", true)
				return response
			}
			currentUser = &User{
				Id:        request.Session.UserID,
				Number:    number,
				BlackList: []int{},
			}
			v.states[currentUser.Id] = &UserState{user: currentUser, state: "root"}
			err = v.mailService.SaveUser(currentUser)
			if err != nil {
				response.Text("Произошла ошибка, попробуйте ещё раз")
				return response
			}

			helloMessage := &Message{From: 1000, To: number, Text: "Добро пожаловать в ряды пользователей Говорящей почты! " +
				"\nЭто первое, приветсвенное 'Hello World' сообщение от создателя навыка. " +
				"\nВы можете использовать номер 1-0-0-0 для отправки ваших отзывов и предложений по навыку. " +
				"\nИногда с этого номера будет приходить важная информация об изменениях в работе навыка. " +
				"\nОтветьте на данное сообщение, если у вас есть идеи, как можно сделать Говорящую почту лучше. " +
				"\nСпасибо, что пользуетесь навыком! " +
				"\nКонец связи."}
			err = v.mailService.SendMessage(helloMessage)
			if err != nil {
				response.Text("Произошла ошибка, попробуйте ещё раз")
				return response
			}

			helloMessage2 := &Message{From: 1000, To: number, Text: "В работе Говорящей почты произошла ошибка, все данные пользователей были утеряны. Я приношу свои иззвинения за это проишествие. Пожалуйста, сообщите мне свой прежний номер, если он не соответсвует новому, я заменю его."}
			err = v.mailService.SendMessage(helloMessage2)
			if err != nil {
				response.Text("Произошла ошибка, попробуйте ещё раз")
				return response
			}

			response.Text(fmt.Sprintf("Добро пожаловать в говорящую почту! Ваш почтовый номер: %s."+
				"\n Поделитесь этим номером с друзьями, и они смогут присылать вам сообщения."+
				"\n Сейчас вы можете отправить новое сообщение или проверить почту, просто скажите об этом."+
				"\n Вы, также, можете завести новые знакомства, отправив сообщение на номер 70-70."+
				"\n И оно достанется случайному пользователю, кто отправил аналогичное сообщение."+
				"\n Если появятся вопросы, скажите - помощь, или задайте вопрос."+
				"\n С чего начнём?", v.printNumber(currentUser.Number)))
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
				text += fmt.Sprintf("У вас %s %s. \nХотите прослушать?", v.printCount(count), alice.Plural(count, "новое сообщение", "новых сообщения", "новых сообщений"))
				v.states[currentUser.Id].state = "ask_start_listen_mail"
				response.Button("Да", "", true)
				response.Button("Нет", "", true)
				response.Button("Помощь", "", true)
			} else {
				text += "У вас нет новых сообщений. Скажите - отправить, чтобы отправить новое сообщение."
				response.Button("Отправить", "", true)
				response.Button("Мой номер", "", true)
				response.Button("Записная книжка", "", true)
				response.Button("Черный список", "", true)
				response.Button("Помощь", "", true)
				response.Button("Мой токен", "", true)
				response.Button("Выйти", "", true)
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
						response.Text(fmt.Sprintf("У вас %s %s. \nХотите прослушать?", v.printCount(count), alice.Plural(count, "новое сообщение", "новых сообщения", "новых сообщений")))
						currentState.state = "ask_start_listen_mail"
						response.Button("Да", "", true)
						response.Button("Нет", "", true)
					} else {
						response.Text("У вас нет новых сообщений.")
						response.Button("Отправить", "", true)
						response.Button("Выйти", "", true)
					}
					return response
				}

				// for send new mail phrase
				if containsIgnoreCase(request.Text(), newMessageWords) {
					currentState.state = "ask_send_number"
					currentState.context = &Message{From: currentUser.Number}
					response.Text("Назовите номер получателя или имя из записной книжки")
					if currentUser.LastNumber > 0 && currentUser.LastNumber != 1000 {
						response.Button(v.printNumber(currentUser.LastNumber), "", true)
					}
					if currentUser.PreLastNumber > 0 && currentUser.PreLastNumber != 1000 {
						response.Button(v.printNumber(currentUser.PreLastNumber), "", true)
					}
					response.Button("Случайное знакомство", "", true)
					response.Button("Оставить отзыв", "", true)
					response.Button("Отмена", "", true)
					return response
				}

				// for my number phrase
				if containsIgnoreCase(request.Text(), myNumberWords) {
					response.Text(fmt.Sprintf("Ваш номер: %s", v.printNumber(currentUser.Number)))
					response.Button("Отправить", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Выйти", "", true)
					return response
				}

				// for my token phrase
				if containsIgnoreCase(request.Text(), myTokenWords) {
					response.Text(fmt.Sprintf("Ваш токен: \n%s", currentUser.Id))
					response.Button("Перейти, чтобы скопировать", "https://yandex.ru/search/?text="+currentUser.Id, false)
					response.Button("Отправить", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Выйти", "", true)
					return response
				}

				// for phone book words
				if containsIgnoreCase(request.Text(), addPhoneBookWord) {
					if currentState.context == nil || currentState.context.To == 0 {
						response.Text("Вы должны отправить сообщение на номер, перед тем как добавить его в записную книжку.")
						response.Button("Отправить", "", true)
						response.Button("Проверить почту", "", true)
						currentState.state = "root"
						return response
					}
					response.Text(fmt.Sprintf("Произнесите имя для номера %s в записной книжке", v.printNumber(currentState.context.To)))
					currentState.state = "ask_phone_username"
					return response
				}

				// for help phrase
				if containsIgnoreCase(request.Text(), helpWords) {
					response.Text("Для того, чтобы отправить сообщение, скажите - отправить. " +
						"\nЧтобы проверить почту, скажите - проверить почту. " +
						"\nЧтобы узнать свой номер, скажите - мой номер. " +
						"\nЧтобы познакомиться с другими пользователями навыка Вы можете отправить сообщение на номер 70-70, или просто скажите \"случайное знакомство\" вместо номера, при отправке сообщения. " +
						"\nЧтобы отменить текущую операцию, скажите - отмена. Скажите - закончить, чтобы выйти из навыка.")
					response.Button("Отправить", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Мой номер", "", true)
					response.Button("Записная книжка", "", true)
					response.Button("Черный список", "", true)
					response.Button("Закончить", "", true)
					return response
				}

				if containsIgnoreCase(request.Text(), clearBlackListWords) {
					currentUser.BlackList = currentUser.BlackList[:0]
					v.mailService.SaveUser(currentUser)

					text := fmt.Sprintf("Черный список был очищен. Хотите проверить почту?")
					response.Text(text)
					currentState.state = "root"
					response.Button("Отправить", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Выйти", "", true)
					return response
				}

				if containsIgnoreCase(request.Text(), blackListWords) {
					var numbers []string
					for i, number := range currentUser.BlackList {
						if i > 15 {
							break
						}
						numbers = append(numbers, v.printNumber(number))
					}
					text := ""
					if len(numbers) > 0 {
						text = fmt.Sprintf("Ваш черный список номеров: "+
							"\n%s"+
							"\nЭти номера не смогут отправлять Вам сообщения. "+
							"\nЧтобы очистить, скажите \"Очистить черный список\"", strings.Join(numbers, "\n"))
					} else {
						text = "Ваш черный список пуст. " +
							"\nДобавить номер в этот список можно только после получения входящего сообщения от пользователя с таким номером."
					}
					response.Text(text)
					currentState.state = "root"
					response.Button("Очистить черный список", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Назад", "", true)
					return response
				}

				if containsIgnoreCase(request.Text(), phoneBookWord) {
					var numbers []string
					i := 0
					for name, number := range currentUser.PhoneBook {
						if i > 15 {
							break
						}
						numbers = append(numbers, fmt.Sprintf("%s : %s", name, v.printNumber(number)))
						i++
					}
					text := ""
					if len(numbers) > 0 {
						text = fmt.Sprintf("Ваша записная книжка номеров: "+
							"\n%s"+
							"\nЧтобы отправить сообщение на эти номера, просто назовите имя. ", strings.Join(numbers, "\n"))
					} else {
						text = "Ваша записная книжка пуста. " +
							"\nДобавить номер в этот список можно только после получения входящего сообщения от пользователя с таким номером."
					}
					response.Text(text)
					currentState.state = "root"
					response.Button("Отправить", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Назад", "", true)
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
				response.Button("Мой номер", "", true)
				response.Button("Записная книжка", "", true)
				response.Button("Черный список", "", true)
				response.Button("Выйти", "", true)
				return response
			}
			if currentState.state == "ask_start_listen_mail" {
				// for yes phrase
				if containsIgnoreCase(request.Text(), acceptWords) {
					message := v.mailService.ReadMessage(currentUser)
					if message == nil {
						response.Text("У вас нет новых сообщений.")
						response.Button("Отправить", "", true)
						response.Button("Выйти", "", true)
						currentState.state = "root"
						return response
					}
					text := fmt.Sprintf("Сообщение от номера: %s. \n%s. \n- \nСлушать дальше или ответить?", v.printNumber(message.From), message.Text)
					response.Text(text)
					currentState.context = message
					currentState.state = "ask_continue_listen_mail"
					response.Button("Дальше", "", true)
					response.Button("Ответить", "", true)
					response.Button("В черный список", "", true)
					response.Button("Отмена", "", true)
					return response
				}

				// for no phrase
				if containsIgnoreCase(request.Text(), negativeWords) || containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState.context = nil

					response.Text("Окей, хотите что-то ещё?")
					response.Button("Отправить", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Мой номер", "", true)
					response.Button("Записная книжка", "", true)
					response.Button("Черный список", "", true)
					response.Button("Нет", "", true)
					return response
				}

				if containsIgnoreCase(request.Text(), helpWords) {
					response.Text("Для того, чтобы отправить сообщение, скажите - отправить. " +
						"\nЧтобы проверить почту, скажите - проверить почту. " +
						"\nЧтобы узнать свой номер, скажите - мой номер. " +
						"\nЧтобы познакомиться с другими пользователями навыка Вы можете отправить сообщение на номер 70-70, или просто скажите \"случайное знакомство\" вместо номера, при отправке сообщения. " +
						"\nЧтобы отменить текущую операцию, скажите - отмена. Скажите - закончить, чтобы выйти из навыка.")
					response.Button("Отправить", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Мой номер", "", true)
					response.Button("Записная книжка", "", true)
					response.Button("Черный список", "", true)
					response.Button("Закончить", "", true)
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
					message := v.mailService.ReadMessage(currentUser)
					if message == nil {
						response.Text("У вас нет новых сообщений.")
						currentState.state = "root"
						response.Button("Отправить", "", true)
						response.Button("Выйти", "", true)
						currentState.context = nil
						return response
					}
					text := fmt.Sprintf("Сообщение от номера: %s. \n%s. \n- \nСлушать дальше или ответить?", v.printNumber(message.From), message.Text)
					response.Text(text)
					currentState.state = "ask_continue_listen_mail"
					currentState.context = message
					response.Button("Дальше", "", true)
					response.Button("Ответить", "", true)
					response.Button("В черный список", "", true)
					response.Button("Отмена", "", true)
					return response
				}

				// for repeat phrase
				if containsIgnoreCase(request.Text(), repeatWords) {
					if currentState.context == nil {
						response.Text("Сообщение для повтора не выбрано.")
						currentState.state = "root"
						return response
					}
					text := fmt.Sprintf("Сообщение от номера: %s. \n%s. \n- \nСлушать дальше или ответить?", v.printNumber(currentState.context.From), currentState.context.Text)
					response.Text(text)
					response.Button("Дальше", "", true)
					response.Button("Ответить", "", true)
					response.Button("В черный список", "", true)
					response.Button("Отмена", "", true)
					return response
				}

				// for no phrase
				if containsIgnoreCase(request.Text(), negativeWords) || containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState.context = nil
					response.Text("Окей, хотите что-то ещё?")
					response.Button("Отправить", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Мой номер", "", true)
					response.Button("Записная книжка", "", true)
					response.Button("Черный список", "", true)
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

				if containsIgnoreCase(request.Text(), clearBlackListWords) {
					currentUser.BlackList = currentUser.BlackList[:0]
					v.mailService.SaveUser(currentUser)

					text := fmt.Sprintf("Черный список был очищен. Хотите проверить почту?")
					response.Text(text)
					currentState.state = "root"
					response.Button("Отправить", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Выйти", "", true)
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
					v.mailService.SaveUser(currentUser)
					text := fmt.Sprintf("Номер %s был добавлен в черный список. \nДля того, чтобы очистить список, просто скажите - очистить черный список. "+
						"Хотите продолжить прослушивание сообщений?", v.printNumber(currentState.context.From))
					response.Text(text)
					currentState.state = "ask_after_black_list"
					return response
				}

				response.Text("Вы можете ответить на это сообщение, сказав - ответить, или продолжить слушать сообщения, просто ответив - дальше. \nТакже, вы можете забанить отправителя сообщения и добавить его в черный список, просто сказав - забанить")
				response.Button("Дальше", "", true)
				response.Button("Ответить", "", true)
				response.Button("Отмена", "", true)
				return response
			}
			if currentState.state == "ask_after_black_list" {
				// for yes phrase
				if containsIgnoreCase(request.Text(), acceptWords) || containsIgnoreCase(request.Text(), nextWords) {
					message := v.mailService.ReadMessage(currentUser)
					if message == nil {
						response.Text("У вас нет новых сообщений.")
						currentState.state = "root"
						return response
					}
					text := fmt.Sprintf("Сообщение от номера %s. \n%s. \n- \nСлушать дальше или ответить?", v.printNumber(message.From), message.Text)
					response.Text(text)
					currentState.state = "ask_continue_listen_mail"
					currentState.context = message
					response.Button("Дальше", "", true)
					response.Button("Ответить", "", true)
					response.Button("В черный список", "", true)
					response.Button("Отмена", "", true)
					return response
				}

				if containsIgnoreCase(request.Text(), clearBlackListWords) {
					currentUser.BlackList = currentUser.BlackList[:0]
					v.mailService.SaveUser(currentUser)

					text := fmt.Sprintf("Черный список был очищен. Хотите проверить почту?")
					response.Text(text)
					currentState.state = "root"
					response.Button("Отправить", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Выйти", "", true)
					return response
				}

				currentState.state = "root"
				response.Text("Окей, хотите что-то ещё?")
				response.Button("Отправить", "", true)
				response.Button("Проверить почту", "", true)
				response.Button("Мой номер", "", true)
				response.Button("Записная книжка", "", true)
				response.Button("Черный список", "", true)
				response.Button("Нет", "", true)
				return response

			}
			if currentState.state == "ask_send_number" {
				// for cancel phrase
				if equalsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState.context = nil
					response.Text("Окей, хотите что-то ещё?")
					response.Button("Отправить новое", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Мой номер", "", true)
					response.Button("Записная книжка", "", true)
					response.Button("Черный список", "", true)
					response.Button("Нет", "", true)
					return response
				}

				if currentState.context == nil {
					response.Text("Скажите - отправить, для того чтобы отправить новое сообщение")
					currentState.state = "root"
					return response
				}
				var to int
				if containsIgnoreCase(request.Text(), reviewWords) {
					to = 1000
				} else if containsIgnoreCase(request.Text(), datingWords) {
					to = 7070
				} else {
					var number string
					tokens := request.Tokens()
					for _, num := range tokens {
						number += num
					}
					var err error
					to, err = strconv.Atoi(number)
					if err != nil {
						if number, ok := currentUser.PhoneBook[strings.ToUpper(request.Text())]; ok {
							to = number
						} else {
							response.Text("Вам нужно назвать четырёхзначный номер получателя или имя из записной книжки. " +
								"\nВы также можете отправить сообщение случайному пользователю на номер 70-70, или оставить отзыв по номеру 1-0-0-0, просто скажите об этом. " +
								"\nСкажите - отмена, чтобы вернуться.")
							response.Button("Отмена", "", true)
							return response
						}
					}
				}
				currentState.context.To = to
				text := fmt.Sprintf("Произнесите текст сообщения")
				if to == 1000 {
					text = fmt.Sprintf("Произнесите текст отзыва или предложения")
				} else if to == 7070 {
					text = fmt.Sprintf("Произнесите текст сообщения для случайного пользователя")
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
					response.Button("Отправить новое", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Мой номер", "", true)
					response.Button("Записная книжка", "", true)
					response.Button("Черный список", "", true)
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
				response.Text(fmt.Sprintf("Отправляю сообщение: \n- \n%s \n- \nНа номер: %s. \nВсё верно?", currentState.context.Text, v.printNumber(currentState.context.To)))
				response.Button("Да", "", true)
				response.Button("Нет", "", true)
				return response

			}
			if currentState.state == "ask_send_confirm" {
				// for yes phrase
				if containsIgnoreCase(request.Text(), acceptWords) || containsIgnoreCase(request.Text(), sendWords) {
					currentState.state = "root"
					err := v.mailService.SendMessage(currentState.context)
					if err != nil {
						response.Text("Произошла ошибка, попробуйте ещё раз")
						response.Button("Отмена", "", true)
						return response
					}
					if currentState.context.To == 1000 {
						currentUser.Reviewed = true
					} else if currentState.context.To == 7070 {
						currentUser.DateFree = true
					} else {
						currentUser.PreLastNumber = currentUser.LastNumber
						currentUser.LastNumber = currentState.context.To
					}

					err = v.mailService.SaveUser(currentUser)
					if err != nil {
						response.Text("Произошла ошибка, попробуйте ещё раз")
						response.Button("Отмена", "", true)
						return response
					}
					if currentState.context.To == 1000 {
						response.Text("Спасибо за отзыв! Вы также можете оставить свой отзыв в Яндекс каталоге навыков.")
						response.Button("Оценить навык", "https://dialogs.yandex.ru/store/skills/eacbce8f-govoryashaya-po", false)
						response.Button("Проверить почту", "", true)
						response.Button("Отправить новое", "", true)
					} else if currentState.context.To != 7070 && phoneBookedNumber(currentUser, currentState.context.To) == nil {
						response.Text("Сообщение отправлено! Вы можете добавить номер в записную книжку. Хотите что то ещё?")
						response.Button("Добавить в записную книжку", "", true)
						response.Button("Проверить почту", "", true)
						response.Button("Отправить новое", "", true)
						response.Button("Нет", "", true)
						return response
					} else {
						response.Text("Сообщение отправлено! Хотите что-то ещё?")
						response.Button("Отправить новое", "", true)
						response.Button("Проверить почту", "", true)
						response.Button("Нет", "", true)
					}
					return response
				}

				// for no phrase
				if containsIgnoreCase(request.Text(), negativeWords) || containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState.context = nil
					response.Text("Окей, хотите что-то ещё?")
					response.Button("Отправить новое", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Мой номер", "", true)
					response.Button("Записная книжка", "", true)
					response.Button("Черный список", "", true)
					response.Button("Нет", "", true)
					return response
				}

				response.Text("Чтобы подтвердить отправку сообщения, скажите - да. \nЛибо скажите - отмена, чтобы вернуться в главое меню")
				response.Button("Да", "", true)
				response.Button("Отмена", "", true)
				return response
			}

			if currentState.state == "ask_phone_username" {
				if currentState.context == nil {
					response.Text("Произошла ошибка, попробуйте ещё раз")
					currentState.state = "root"
					response.Button("Отправить новое", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Выйти", "", true)
					return response
				}
				// for yes phrase
				if containsIgnoreCase(request.Text(), datingWords) || containsIgnoreCase(request.Text(), reviewWords) {
					response.Text("Вы не можете использовать это имя, пожалуйста, назовите другое.")
					response.Button("Отмена", "", true)
					return response
				}

				// for no phrase
				if containsIgnoreCase(request.Text(), negativeWords) || containsIgnoreCase(request.Text(), cancelWords) {
					currentState.state = "root"
					currentState.context = nil
					response.Text("Окей, хотите что-то ещё?")
					response.Button("Отправить новое", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Мой номер", "", true)
					response.Button("Записная книжка", "", true)
					response.Button("Черный список", "", true)
					response.Button("Нет", "", true)
					return response
				}

				if request.Text() != "" {
					currentUser.PhoneBook[strings.ToUpper(request.Text())] = currentState.context.To
					err := v.mailService.SaveUser(currentUser)
					if err != nil {
						response.Text("Произошла ошибка, попробуйте ещё раз")
						response.Button("Выйти", "", true)
						return response
					}
					response.Text(fmt.Sprintf("Для номера: %s, установлено имя: %s, вы можете использовать его для отправки сообщений. \nХотите что то ещё?", v.printNumber(currentState.context.To), request.Text()))
					response.Button("Отправить новое", "", true)
					response.Button("Проверить почту", "", true)
					response.Button("Выйти", "", true)
					currentState.context = nil
					currentState.state = "root"
					return response
				}

				response.Text(fmt.Sprintf("Назовите имя для номера - %s", v.printNumber(currentState.context.To)))
				response.Button("Да", "", true)
				response.Button("Отмена", "", true)
				return response
			} else {
				v.states[currentUser.Id] = &UserState{user: currentUser, state: "root"}
				response.Text("Что пожелаете?")
				response.Button("Отправить новое сообщение", "", true)
				response.Button("Проверить почту", "", true)
				response.Button("Помощь", "", true)
				return response
			}
		}

		response.Text("Произошла ошибка, попробуйте позже")
		response.Button("Проверить почту", "", true)
		response.Button("Закончить", "", true)
		return response
	}
}

func phoneBookedNumber(user *User, to int) *string {
	for name, number := range user.PhoneBook {
		if number == to {
			return &name
		}
	}
	return nil
}

func (v VoiceMail) getCountOfMessages(currentUser *User) int {
	return len(v.mailService.GetMessagesForUser(currentUser))
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
	number, err, done := v.mailService.checkAndGenerateId(number)
	if done {
		return number, err
	}
	number = 10000 + rand.Intn(99999-10000)
	number, err, done = v.mailService.checkAndGenerateId(number)
	if done {
		return number, err
	}
	return 0, errors.New("COLLISION error when generating unique id")
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
