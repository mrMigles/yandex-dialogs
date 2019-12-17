package voice_mail

import (
	"log"
	"math/rand"
	"time"
)

type DatingBot struct {
	mailService *MailService
}

func NewDatingBot(service *MailService) DatingBot {
	return DatingBot{mailService: service}
}

func (m DatingBot) CheckMails() {
	log.Print("Run Dating cron")
	sentMessages := map[int]map[int]struct{}{}
	messages := m.mailService.GetMessagesForUser(&User{Number: 7070})
	freeDateUsers := m.mailService.GetDateFreeUsers()
	rand.Seed(time.Now().UnixNano())
	for _, message := range messages {
		rand.Shuffle(len(freeDateUsers), func(i, j int) { freeDateUsers[i], freeDateUsers[j] = freeDateUsers[j], freeDateUsers[i] })
		for _, user := range freeDateUsers {
			if message.From != user.Number {
				if from, ok := sentMessages[message.From]; ok {
					if _, ok := from[user.Number]; ok {
						continue
					}
				}
				message.To = user.Number
				err := m.mailService.SendMessage(&message)
				if err != nil {
					log.Printf("Error with loop %v", err)
					continue
				}
				if _, ok := sentMessages[message.From]; !ok {
					sentMessages[message.From] = map[int]struct{}{}
				}
				sentMessages[message.From][message.To] = struct{}{}
			}
		}
	}
}

func (m DatingBot) GetCron() string {
	return "0 * * * *"
}
