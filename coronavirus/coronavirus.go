package coronavirus

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/azzzak/alice"
	"github.com/go-bongo/bongo"
	"github.com/patrickmn/go-cache"
	"gopkg.in/mgo.v2/bson"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"yandex-dialogs/common"
)

var mongoConnection = common.GetEnv("COMMON_MONGO_CONNECTION", "")
var databaseName = common.GetEnv("COMMON_DATABASE_NAME", "common")
var statusCache = cache.New(5*time.Minute, 10*time.Minute)

var shortPhrases = "Число заразившихся на сегодняшний день достигло %d %s, умерли %d %s."

var funWords = []string{"когда", "эпидемия", "консервы"}
var yesWord = "да"
var acceptNews = []string{"давай", "можно", "плюс", "ага", "угу", "дэ", "новости", "что там в мире", "что в мире", "Да, давай новости", "давай новости"}
var helpWords = []string{"помощь", "что ты може", "что ты умеешь"}
var cancelWords = []string{"отмена", "хватит", "все", "всё", "закончи", "закончить", "выход", "выйди", "выйти"}
var protectWords = []string{"защитить", "что делать", "не заболеть", "чеснок", "боротьс", "нужно", "как "}
var symptomsWords = []string{"симптом", "чувств", "заболел", "плохо", "болею"}
var masksWords = []string{"маск", "респератор", "защита"}

var runSkillPhrases = []string{"Хрен знает, выживший, на кой ляд тебе этот коронавирус сдался, но я в чужие дела не лезу.", "Здравствуй, выживший!", "Здравствуй, выживший!", "Здравствуй, выживший!", "Поздравляю, вы всё ещё живы! А тем временем", "Добро, выживший!", "Приветствую, выживший!", "Приветствую, выживший!", "Приветствую, выживший!", "Приветствую, выживший!"}
var endSkillPhrases = []string{"Удачи, выживший!", "Ну бывай, выживший!", "Не хворай, выживший!", "Не болей, выживший!"}
var newsPhrases = []string{"Хочешь послушать полную сводку или услышать про симптомы?", "Послушаешь подробно новости или рассказать о симптомах?", "Рассказать новости или послушаешь как защититься от вируса?"}

var howToProtectPhrases = []string{"Всемирная организация здравоохранения рекомендует следующие" +
	" меры, которые защищают от многих вирусов:	" +
	"\n - Правильно и регулярно мойте руки: не меньше 20 секунд, с мылом и тщательно промывая все участки, а затем вытирайте насухо. " +
	"Если мыла под рукой нет, можно использовать антисептический гель." +
	"\n - Не прикасайтесь грязными руками к лицу, особенно носу, рту и глазам." +
	"\n - Не приближайтесь к людям, которые кашляют и чихают, а также к тем, у кого высокая температура." +
	"\n - Готовьте мясо и яйца как положено, то есть при достаточной температуре.",

	"Чтобы минимизировать риски заражения вирусом, медики рекомендуют:	" +
		"\n - Правильно и регулярно мойте руки: не меньше 20 секунд, с мылом и тщательно промывая все участки, а затем вытирайте насухо. " +
		"Если мыла под рукой нет, можно использовать антисептический гель." +
		"\n - Не прикасайтесь грязными руками к лицу, особенно носу, рту и глазам." +
		"\n - Не приближайтесь к людям, которые кашляют и чихают, а также к тем, у кого высокая температура." +
		"\n - Готовьте мясо и яйца как положено, то есть при достаточной температуре."}

var symptomsPhrases = []string{"Симптомы во многом сходны со многими респираторными заболеваниями, часто имитируют обычную простуду, могут походить на грипп. " +
	"\n - Чувство усталости. " +
	"\n - Затруднённое дыхание. " +
	"\n - Высокая температура. " +
	"\n - Кашель и / или боль в горле. " +
	"\n Если у вас есть аналогичные симптомы, подумайте о следующем: " +
	"\n - Вы посещали в последние две недели в зоны повышенного риска (Китай и прилегающие регионы)? " +
	"\n - Вы были в контакте с кем-то, кто посещал в последние две недели в зоны повышенного риска (Китай и прилегающие регионы)? " +
	"\n - Если ответ на эти вопросы положителен - к симптомам следует отнестись максимально внимательно. "}

var masksPhrases = []string{"Теоретически вряд ли маски очень полезны. Недостатков у них очень много. Но если всё таки хочется их носить, соблюдайте следующие правила:" +
	"\n - Аккуратно закройте нос и рот маской и закрепите её, чтобы уменьшить зазор между лицом и маской." +
	"\n - Не прикасайтесь к маске во время использования. После прикосновения к использованной маске, например, чтобы снять её, вымойте руки." +
	"\n - После того, как маска станет влажной или загрязнённой, наденьте новую чистую и сухую маску." +
	"\n - Не используйте повторно одноразовые маски. Их следует выбрасывать после каждого использования и утилизировать сразу после снятия."}

var defaultAnswer = &DayStatus{
	Short:  "Выживший... Сервера пали... Связи больше нет.",
	News:   "Хрен знает на кой ляд тебе эти новости сдались, но я в чужие дела не лезу, хочешь, значит есть зачем... только вот сервера всё равно недоступны.",
	Status: []string{"Скорее всего апокалипсис уже наступил."},
}

type DayStatus struct {
	bongo.DocumentBase `bson:",inline"`
	Short              string   `json:"-"`
	Cases              int      `json:"-,"`
	Death              int      `json:"-,"`
	News               string   `json:"-,"`
	Status             []string `json:"-,"`
}

type CountryInfo struct {
	Region string `json:"region"`
	Cases  string `json:"cases"`
	Death  string `json:"death"`
}

type Coronavirus struct {
	mux        sync.Mutex
	connection *bongo.Connection
	httpClient http.Client
}

func NewCoronavirus() Coronavirus {
	rand.Seed(time.Now().Unix())
	config := &bongo.Config{
		ConnectionString: mongoConnection,
		Database:         databaseName,
	}
	connection, err := bongo.Connect(config)
	if err != nil {
		log.Fatal(err)
	}
	connection.Session.SetPoolLimit(50)
	return Coronavirus{
		connection: connection,
		httpClient: http.Client{Timeout: time.Millisecond * 2000},
	}
}

func (c Coronavirus) GetPath() string {
	return "/api/dialogs/coronavirus"
}

func (c Coronavirus) Health() (result bool, message string) {
	if c.Ping() != nil {
		log.Printf("Ping failed")
		c.Reconnect()
		return false, "DB is not available"
	}
	return true, "OK"
}

func (c Coronavirus) Reconnect() {
	err := c.connection.Connect()
	if err != nil {
		log.Print(err)
	}
	c.connection.Session.SetPoolLimit(50)
}

func (c Coronavirus) Ping() (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Print("Recovered in f", r)
			err = errors.New("Error ping")
		}
	}()
	err = c.connection.Session.Ping()
	return err
}

func (c Coronavirus) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	return func(request *alice.Request, response *alice.Response) (resp *alice.Response) {
		defer func() {
			if r := recover(); r != nil {
				log.Print("Recovered in f: ", r)
				response.Text("Произошла ошибка, попробуйте в другой раз")
				response.Button("Закончить", "", true)
				resp = response
			}
		}()
		c.Health()

		currentStatus := c.GetDayStatus()

		text := ""

		if request.IsNewSession() {
			text += runSkillPhrases[rand.Intn(len(runSkillPhrases))]
			text += " "
		}

		if containsIgnoreCase(request.Text(), helpWords) {
			response.Text("Это твой личный гид в хроники коронавируса. Полезная хреновина, которая помогает подготовиться на случай возможной эпидемии. А если и так, то хоть будешь знать, когда консервы покупать, хе-хе-хе... " +
				"\nПросто слушай сводку за день и следуй указаниям навыка." +
				"\nМожешь спросить меня о симптомах коронависа или о том, как от него защититься.")
			response.Button("Выйти", "", true)
			return response
		}

		if strings.EqualFold(request.Text(), yesWord) || containsIgnoreCase(request.Text(), acceptNews) {
			text += currentStatus.News
			response.Text(text)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), funWords) {
			text += currentStatus.Status[rand.Intn(len(currentStatus.Status))]
			text += " А вообще: "
			text += currentStatus.Short

			response.Text(text)
			response.Button("Хроники коронавируса", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), cancelWords) {
			text := endSkillPhrases[rand.Intn(len(endSkillPhrases))]
			response.Text(text + " Скажи - закончить, чтобы я отключился.")
			response.Button("Оценить навык", "https://dialogs.yandex.ru/store/skills/d5087c0d-hroniki-koronavirusa", false)
			response.Button("Закончить", "", false)
			return response
		}

		if containsIgnoreCase(request.Text(), symptomsWords) {
			text += symptomsPhrases[rand.Intn(len(symptomsPhrases))]
			response.Text(text)
			return response
		}

		if containsIgnoreCase(request.Text(), protectWords) {
			text += howToProtectPhrases[rand.Intn(len(howToProtectPhrases))]
			response.Text(text)
			return response
		}

		if containsIgnoreCase(request.Text(), masksWords) {
			text += masksPhrases[rand.Intn(len(masksPhrases))]
			response.Text(text)
			return response
		}

		if text == "" {
			text += runSkillPhrases[rand.Intn(len(runSkillPhrases))]
		}
		text += " "
		text += currentStatus.Short
		text += " \n"
		text += currentStatus.Status[rand.Intn(len(currentStatus.Status))]
		text += " \n"
		text += newsPhrases[rand.Intn(len(newsPhrases))]
		response.Text(text)
		response.Button("Да, давай новости", "", false)
		response.Button("Симптомы", "", false)
		response.Button("Как защититься", "", false)
		response.Button("Выйти", "", false)
		return response
	}
}

func (c Coronavirus) GetDayStatus() *DayStatus {
	status := &DayStatus{}
	err := c.connection.Collection("coronavirus").FindOne(bson.M{}, status)
	if err != nil {
		return defaultAnswer
	}

	var countryInfos []CountryInfo
	resp, err := c.httpClient.Get("https://coronavirus.zone/data.json")
	if err != nil {
		return c.buildErrorStatus(status)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return c.buildErrorStatus(status)
	}

	err = json.Unmarshal(bodyBytes, &countryInfos)
	if err != nil {
		return c.buildErrorStatus(status)
	}

	cases := 0
	death := 0
	for _, info := range countryInfos {
		caseVar, _ := strconv.Atoi(info.Cases)
		cases += caseVar

		deathVar, _ := strconv.Atoi(info.Death)
		death += deathVar
	}

	if cases > 0 && death > 0 {
		status.Short = fmt.Sprintf(shortPhrases, cases, alice.Plural(cases, "человек", "человек", "человек"), death, alice.Plural(cases, "человек", "человека", "человек"))

		if status.Cases != cases || status.Death != death {
			status.Death = death
			status.Cases = cases
			err = c.connection.Collection("coronavirus").Save(status)
			if err != nil {
				log.Print("Error when saving to DB")
			}
		}
		return status
	} else {
		return defaultAnswer
	}
}

func (c Coronavirus) buildErrorStatus(status *DayStatus) *DayStatus {
	if status.Cases > 0 && status.Death > 0 {
		status.Short = fmt.Sprintf(shortPhrases, status.Cases, alice.Plural(status.Cases, "человек", "человек", "человек"), status.Death, alice.Plural(status.Death, "человек", "человека", "человек"))
		return status
	} else {
		return defaultAnswer
	}
}

func containsIgnoreCase(message string, wordsToCheck []string) bool {
	for _, word := range wordsToCheck {
		if strings.Contains(strings.ToUpper(message), strings.ToUpper(word)) {
			return true
		}
	}
	return false
}
