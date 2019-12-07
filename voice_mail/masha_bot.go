package voice_mail

import (
	"log"
	"strconv"
	"yandex-dialogs/masha"
)

type MashaBot struct {
	mailService MailService
	mashaSkill  masha.Masha
}

func NewMashaBot(service MailService) MashaBot {
	mashaSkill := masha.NewMasha(5000)
	return MashaBot{mailService: service, mashaSkill: mashaSkill}
}

func (m MashaBot) CheckMails() {
	log.Print("Run Masha cron")
	messages := m.mailService.GetMessagesForUser(&User{Number: 8800})
	for _, message := range messages {
		log.Printf("Cron message from %d", message.From)
		question := message.Text
		answer, err := m.mashaSkill.GetAnswer(strconv.Itoa(message.From), question)
		if err != nil {
			log.Print(err)
			continue
		}
		answerMessage := &Message{To: message.From, From: 8800, Text: answer}
		err = m.mailService.SendMessage(answerMessage)
		if err != nil {
			log.Print(err)
			continue
		}
		err = m.mailService.DeleteMessage(&message)
		if err != nil {
			log.Printf("Error: %v", err)
		}
	}
}

func (m MashaBot) GetCron() string {
	return "*/5 * * * *"
}
