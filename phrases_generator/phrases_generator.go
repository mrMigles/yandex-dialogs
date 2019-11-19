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
	"yandex-dialogs/common"
)

var client = http.Client{}

type PhrasesGenerator struct {
	states map[string]State
	mux    sync.Mutex
	apiUrl string
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
		apiUrl: common.GetEnv("TITLE_GENERATOR_URL", ""),
	}
}

func (v PhrasesGenerator) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) *alice.Response {

		if request.Session.New == true {
			currentState := State{
				action: "ask",
				word:   "",
				last:   "",
			}
			//v.mux.Lock()
			//defer v.mux.Unlock()
			v.states[request.Session.UserID] = currentState
			response.Text("Здравствуйте! Просто произнесите слово, и я придумаю заголовок с этим словом.")
			return response
		}

		if strings.Contains(request.Text(), "помощь") || strings.Contains(request.Text(), "ты умеешь") {
			currentState := State{
				action: "ask",
				word:   "",
				last:   "",
			}
			//v.mux.Lock()
			//defer v.mux.Unlock()
			v.states[request.Session.UserID] = currentState
			response.Text("Я могу придумывать заголовки для названного слова. Для того, чтобы начать просто назовите" +
				" слово. Если вы хотите прослушать заголовок ещё раз, просто скажите - повтори, если хотите услышать другой " +
				"заголовок к вашему слову, то скажите - ещё, а если хотите указать новое слово, то скажите - новое слово. " +
				"Когда надоест, просто скажите - хватит.")
			return response
		}

		if strings.Contains(request.Text(), "хватит") || strings.Contains(request.Text(), "всё") {
			delete(v.states, request.Session.UserID)
			response.Text("Заходите ещё.")
			response.Response.EndSession = true
			return response
		}

		if currentState, ok := v.states[request.Session.UserID]; ok {

			if strings.Contains(request.Text(), "ещё") || strings.Contains(request.Text(), "еще") || strings.Contains(request.Text(), "друго") {
				if currentState.action == "ans" {
					answer := v.getAnswer(currentState.word)
					response.Text(answer)
					currentState = State{
						action: "ans",
						word:   currentState.word,
						last:   answer,
					}
					v.states[request.Session.UserID] = currentState
					return response
				} else {
					currentState := State{
						action: "ask",
						word:   "",
						last:   "",
					}
					//v.mux.Lock()
					//defer v.mux.Unlock()
					v.states[request.Session.UserID] = currentState
					response.Text("Произнесите слово, и я придумаю заголовок.")
					return response
				}
			}

			if strings.Contains(request.Text(), "повтори") || strings.Contains(request.Text(), "не понял") {
				if currentState.action == "ans" {
					response.Text(currentState.last)
					return response
				} else {
					currentState := State{
						action: "ask",
						word:   "",
						last:   "",
					}
					//v.mux.Lock()
					//defer v.mux.Unlock()
					v.states[request.Session.UserID] = currentState
					response.Text("Произнесите слово, и я придумаю заголовок.")
					return response
				}
			}

			if strings.Contains(request.Text(), "новое") || strings.Contains(request.Text(), "новый") {
				currentState := State{
					action: "ask",
					word:   "",
					last:   "",
				}
				//v.mux.Lock()
				//defer v.mux.Unlock()
				v.states[request.Session.UserID] = currentState
				response.Text("Произнесите слово, и я придумаю заголовок.")
				return response
			}

			if currentState.action == "ask" {
				answer := v.getAnswer(request.Text())
				response.Text(answer)
				currentState = State{
					action: "ans",
					word:   request.Text(),
					last:   answer,
				}
				v.states[request.Session.UserID] = currentState
				return response
			} else {
				currentState = State{
					action: "ask",
					word:   "",
					last:   "",
				}
				v.states[request.Session.UserID] = currentState
				response.Text("Произнесите новое слово")
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

	}
}

func (v PhrasesGenerator) getAnswer(text string) string {
	resp, err := http.PostForm(
		v.apiUrl,
		url.Values{
			"moduleName": {"TitleGen"},
			"cmd":        {"gen"},
			"word":       {text},
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
	return out
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
