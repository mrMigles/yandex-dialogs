package voice_mail

import (
	"github.com/go-bongo/bongo"
	"gopkg.in/mgo.v2/bson"
	"log"
	"yandex-dialogs/common"
)

var mongoConnection = common.GetEnv("MONGO_CONNECTION", "")
var databaseName = common.GetEnv("DATABASE_NAME", "voice-mail")
var encryptKey = common.GetEnv("ENCRYPT_KEY", "")

type MailService struct {
	connection *bongo.Connection
}

func NewMailService() MailService {
	config := &bongo.Config{
		ConnectionString: mongoConnection,
		Database:         databaseName,
	}
	connection, err := bongo.Connect(config)

	if err != nil {
		log.Fatal(err)
	}
	return MailService{connection: connection}
}

func (m MailService) Reconnect() {
	log.Printf("Reconnect")
	m.connection.Session.Close()
	config := &bongo.Config{
		ConnectionString: mongoConnection,
		Database:         databaseName,
	}
	m.connection, _ = bongo.Connect(config)
}

func (m MailService) Ping() error {
	return m.connection.Session.Ping()
}

func (m MailService) SaveUser(user *User) error {
	return m.connection.Collection("users").Save(user)
}

func (m MailService) SendMessage(message *Message) error {
	return m.connection.Collection("messages").Save(message)
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

func (m MailService) findUser(userId string) *User {
	user := &User{}
	err := m.connection.Collection("users").FindOne(bson.M{"id": userId}, user)

	if err != nil {
		log.Printf("User %s not found", userId)
		return nil
	} else {
		log.Printf("Found user: %+v", user)
	}
	return user
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
