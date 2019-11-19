package phrases_generator

import (
	"github.com/azzzak/alice"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

var client = http.Client{}

type PhrasesGenerator struct {
	states map[string]State
	mux    sync.Mutex
}

type State struct {
	action string
	word   string
	last   string
}

func (v PhrasesGenerator) GetPath() string {
	return "/api/dialogs/phrases-generator"
}

func NewDialog() PhrasesGenerator {
	return PhrasesGenerator{
		states: map[string]State{},
	}
}

func (v PhrasesGenerator) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) *alice.Response {

		if currentState, ok := v.states[request.Session.UserID]; ok {
			if currentState.action == "ask" {
				resp, err := http.PostForm(
					"http://title.web-canape.ru/ajax/ajax.php",
					url.Values{
						"moduleName": {"TitleGen"},
						"cmd":        {"gen"},
						"word":       {request.Text()},
						"language":   {"ru"},
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

				out, _ := getStringInBetween(bodyString, "<div class=\\\"js-full_text\\\" style=\\\"display: none;\\\">", "<\\/div>")
				response.Text(out)
				currentState = State{
					action: "ans",
					word:   request.Text(),
					last:   out,
				}
				v.states[request.Session.UserID] = currentState
				return response
			}
		} else {
			currentState := State{
				action: "ask",
				word:   "",
				last:   "",
			}
			//v.mux.Lock()
			//defer v.mux.Unlock()
			v.states[request.Session.UserID] = currentState
			response.Text("Здравствуйте! Просто произнесите слово, и я придумаю заголовок.")
			return response
		}
		response.Text(request.OriginalUtterance())
		return response
	}
}

func getStringInBetween(str string, start string, end string) (string, error) {
	s := strings.Index(str, start)
	if s == -1 {
		return "", nil
	}
	s += len(start)
	str = str[s:]
	e := strings.Index(str, end)
	if e == -1 {
		return "", nil
	}
	str = str[0:e]
	return strconv.Unquote("\"" + str + "\"")
}
