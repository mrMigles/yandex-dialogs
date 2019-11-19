package masha

import (
	"github.com/azzzak/alice"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

type Masha struct {
}

func NewMasha() Masha {
	return Masha{}
}

func (v Masha) GetPath() string {
	return "/api/dialogs/masha"
}

func (v Masha) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) *alice.Response {
		resp, err := http.PostForm(
			"https://masha-bot.herokuapp.com/api/v1/masha/answer",
			url.Values{
				"chatId":  {request.Session.UserID},
				"message": {request.Text()},
			},
		)

		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		bodyString := string(bodyBytes)

		response.Text(bodyString)
		return response
	}
}

func (v Masha) getAnswer(text string, userId string) {

}
