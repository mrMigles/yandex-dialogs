package voice_mail

import (
	"errors"
	"github.com/go-bongo/bongo"
	"gopkg.in/mgo.v2/bson"
	"log"
	"net"
	"yandex-dialogs/common"
)

var mongoConnection = common.GetEnv("MONGO_CONNECTION", "")
var databaseName = common.GetEnv("DATABASE_NAME", "voice-mail")
var encryptKey = common.GetEnv("ENCRYPT_KEY", "")

type MailService struct {
	connection *bongo.Connection
}

func NewMailService() *MailService {
	config := &bongo.Config{
		ConnectionString: mongoConnection,
		Database:         databaseName,
	}
	connection, err := bongo.Connect(config)
	if err != nil {
		log.Fatal(err)
	}
	connection.Session.SetPoolLimit(50)
	return &MailService{connection: connection}
}

func (m MailService) Reconnect() {
	err := m.connection.Connect()
	if err != nil {
		log.Print(err)
	}
	m.connection.Session.SetPoolLimit(50)
}

func (m MailService) Ping() (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("Recovered in f", r)
			err = errors.New("Error ping")
		}
	}()
	err = m.connection.Session.Ping()
	return err
}

func (m MailService) SaveUser(user *User) error {
	return m.connection.Collection("users").Save(user)
}

func (m MailService) SendMessage(message *Message) error {
	toUser, _ := m.findUserByNumber(message.To)
	if toUser == nil && message.To != 7070 && message.To != 8800 && message.To != 1000 {
		log.Printf("Message from user %d didn't send to user %d because user doesn't exist", message.From, message.To)
		return nil
	}
	if toUser != nil && contains(toUser.BlackList, message.From) {
		log.Printf("Message from user %d didn't send to user %d because of blacklist", message.From, message.To)
		return nil
	}
	return m.connection.Collection("messages").Save(message)
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (m MailService) ReadMessage(user *User) *Message {
	message := &Message{}
	err := m.connection.Collection("messages").FindOne(bson.M{"to": user.Number}, message)

	if err != nil {
		log.Printf("Messages for user %d not found", user.Number)
		return nil
	}

	err = m.connection.Collection("messages").DeleteDocument(message)
	if err != nil {
		log.Printf("Error: %v", err)
	}
	log.Printf("Found message %v for user %d. Removed.", message.GetId(), user.Number)

	return message
}

func (m MailService) GetMessagesForUser(user *User) []Message {
	results := m.connection.Collection("messages").Find(bson.M{"to": user.Number})
	var messages []Message
	message := &Message{}
	for results.Next(message) {
		messages = append(messages, *message)
	}
	return messages
}

func (m MailService) findUser(userId string) (*User, error) {
	user := &User{}
	err := m.connection.Collection("users").FindOne(bson.M{"id": userId}, user)

	if err != nil {
		if _, ok := err.(*net.OpError); ok {
			return nil, err
		}
		log.Printf("User %s not found", userId)
		return nil, nil
	} else {
		log.Printf("Found user: %+v", user)
	}
	return user, nil
}

func (m MailService) findUserByNumber(number int) (*User, error) {
	user := &User{}
	err := m.connection.Collection("users").FindOne(bson.M{"number": number}, user)

	if err != nil {
		if _, ok := err.(*net.OpError); ok {
			return nil, err
		}
		log.Printf("User %d not found", number)
		return nil, nil
	} else {
		log.Printf("Found user: %+v", user)
	}
	return user, nil
}

func (m MailService) GetDateFreeUsers() []User {
	results := m.connection.Collection("users").Find(bson.M{"datefree": true})
	var users []User
	user := &User{}
	for results.Next(user) {
		users = append(users, *user)
	}
	return users
}

func (m MailService) GetReviewUsers() []User {
	results := m.connection.Collection("users").Find(bson.M{"reviewed": true})
	var users []User
	user := &User{}
	for results.Next(user) {
		users = append(users, *user)
	}
	return users
}

func (m MailService) checkAndGenerateId(number int) (int, error, bool) {
	for i := 0; i < 10; i++ {
		user := &User{}
		err := m.connection.Collection("users").FindOne(bson.M{"number": number}, user)
		if err != nil {
			if _, ok := err.(*bongo.DocumentNotFoundError); ok {
				return number, nil, true
			} else {
				log.Print("real error " + err.Error())
				return 0, err, true
			}
		}
		number++
	}
	return 0, nil, false
}

func (m MailService) DeleteMessage(message *Message) error {
	return m.connection.Collection("messages").DeleteDocument(message)
}
