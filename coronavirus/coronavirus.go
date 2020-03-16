package coronavirus

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/azzzak/alice"
	"github.com/go-bongo/bongo"
	"github.com/robfig/cron/v3"
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
var coronavirusApi = common.GetEnv("CORONAVIRUS_API", "")

var fullFirstPhrase = "На сегодняшний день в мире зафиксировано %d %s заражения коронавирусной инфекцией%s. \n%d %s умерли от болезни%s. \nВыздоровевших - %d %s. \nОсновные очаги заражения: %s. \nВ России количество заразившихся достигло %d %s%s."
var epicentr = "Вот 20 стран, с наибольшим количеством заразившихся: %s\n"
var moreThanYesterday = ", это на %d больше, чем вчера"
var moreThenDay = ", за сутки это число увеличилось на %d"
var moreThanLastDay = ", их количество выросло на %d за последний день"

var countryInfo = "В регионе \"%s\" было зафиксировано %d %s заражения%s. \n%d %s умерли от болезни%s. \nВыздоровевших - %d %s%s."
var countryInfoWithoutY = "В регионе \"%s\" было зафиксировано %d %s заражения. \n%d %s умерли от болезни. \nВыздоровевших - %d %s."

var funWords = []string{"когда", "эпидемия", "консерв"}
var statsWords = []string{"статистик", "стран", "город", "област"}
var yesWord = "да"
var yesterdayNews = []string{"вчера", "прошлы"}
var epicentrWords = []string{"очаг", "самый", "самое", "заразивших", "заражен"}
var acceptNews = []string{"давай", "можно", "плюс", "ага", "угу", "дэ", "новости", "что там в мире", "что в мире", "Да, давай новости", "давай новости"}
var helpWords = []string{"помощь", "что ты може", "что ты умеешь"}
var cancelWords = []string{"отмена", "хватит", "все", "всё", "закончи", "закончить", "выход", "выйди", "выйти"}
var protectWords = []string{"защитить", "что делать", "не заболеть", "чеснок", "боротьс", "нужно", "как "}
var symptomsWords = []string{"симптом", "чувств", "заболел", "плохо", "болею"}
var masksWords = []string{"маск", "респератор", "защита"}

var runSkill = []string{"коронавирус", "хроник"}

var runSkillPhrases = []string{"Здравствуйте!", "Приветствую!"}
var endSkillPhrases = []string{"Удачи Вам, выживший! Постарайтесь сократить возможные контакты с зараженными и чаще мойте руки.", "Не хворайте, выживший! Постарайтесь сократить возможные контакты с зараженными и чаще мойте руки.", "Не болейте, выживший! Постарайтесь сократить возможные контакты с зараженными и чаще мойте руки."}
var newsPhrases = []string{"Хотите прослушать новости, посмотреть статистику заражений или услышать про симптомы?", "Послушаете новости, статистику заражений или рассказать о симптомах?", "Рассказать новости, статистику заражений или послушаете как защититься от вируса?"}
var firstHi = "Вы можете узнать статистику заболевания в определенной стране, регионе или городе, либо услышать статистику по очагам заболевания, прослушать актуальные новости, а также узнать информацию по симптомам болезни и методам защиты от вируса."

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
	"\n - Кашель или боль в горле. " +
	"\n Если у вас есть аналогичные симптомы, подумайте о следующем: " +
	"\n - Вы посещали в последние две недели в зоны повышенного риска (это Китай, Южная Корея, Италия или другие страны с эпидемией)? " +
	"\n - Вы были в контакте с кем-то, кто посещал в последние две недели в зоны повышенного риска? " +
	"\nЕсли ответ на эти вопросы положителен - к симптомам следует отнестись максимально внимательно, постарайтесь незамедлительно обратиться за медицинской помощью. "}

var masksPhrases = []string{"Теоретически, вряд ли маски очень полезны. Недостатков у них очень много. Но если всё таки хочется их носить, соблюдайте следующие правила:" +
	"\n - Аккуратно закройте нос и рот маской и закрепите её, чтобы уменьшить зазор между лицом и маской." +
	"\n - Не прикасайтесь к маске во время использования. После прикосновения к использованной маске, например, чтобы снять её, вымойте руки." +
	"\n - После того, как маска станет влажной или загрязнённой, наденьте новую чистую и сухую маску." +
	"\n - Не используйте повторно одноразовые маски. Их следует выбрасывать после каждого использования и утилизировать сразу после снятия."}

type DayStatus struct {
	bongo.DocumentBase `bson:",inline"`
	Current            CoronavirusInfo `json:"current"`
	Yesterday          CoronavirusInfo `json:"yesterday"`
}

type CoronavirusInfo struct {
	Countries     []Region `json:"regions"`
	Cities        []Region `json:"cities"`
	Confirmed     int      `json:"confirmed"`
	Deaths        int      `json:"deaths"`
	Cured         int      `json:"cured"`
	ImportantNews []New    `json:"importantNews"`
	AllNews       []New    `json:"allNews"`
}

type Response struct {
	Cities        CitiesContainer        `json:"russianSubjects"`
	Countries     CountriesContainer     `json:"countries"`
	AllNews       AllNewsContainer       `json:"allNews"`
	ImportantNews ImportantNewsContainer `json:"importantNews"`
}

type CitiesContainer struct {
	Data DataCities `json:"data"`
}

type DataCities struct {
	Cities []Region `json:"subjects"`
}

type CountriesContainer struct {
	Data DataCountries `json:"data"`
}

type DataCountries struct {
	Countries []Region `json:"countries"`
}

type AllNewsContainer struct {
	Data DataAllNews `json:"data"`
}

type DataAllNews struct {
	AllNews []New `json:"news"`
}

type ImportantNewsContainer struct {
	Data DataImportantNews `json:"data"`
}

type DataImportantNews struct {
	ImportantNews []New `json:"news"`
}

type New struct {
	Important bool   `json:"important,omitempty"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	Url       string `json:"url"`
}

type Region struct {
	Ru        string `json:"ru"`
	Confirmed int    `json:"confirmed"`
	Deaths    int    `json:"deaths"`
	Cured     int    `json:"cured"`
	IsCountry bool   `json:"isCountry,omitempty"`
}

type User struct {
	bongo.DocumentBase `bson:",inline"`
	Id                 string `json:"-,"`
	Count              int    `json:"count,"`
}

type Coronavirus struct {
	backupStatus *DayStatus
	mux          sync.Mutex
	connection   *bongo.Connection
	httpClient   http.Client
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
	coronavirus := Coronavirus{
		connection: connection,
		httpClient: http.Client{Timeout: time.Millisecond * 20000},
	}
	coronavirus.backupStatus = coronavirus.grabData()
	c := cron.New()
	c.AddFunc("*/5 * * * *", func() {
		coronavirus.backupStatus = coronavirus.grabData()
	})
	c.Start()
	return coronavirus
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
		user := c.GetUser(request.UserID())
		if user == nil {
			user = &User{
				Id:    request.UserID(),
				Count: 0,
			}
		}
		user.Count++
		c.saveUser(user)

		text := ""

		if currentStatus == nil {
			currentStatus = c.backupStatus
			if currentStatus == nil {
				response.Text("В работе навыка произошли проблемы, пожалуйста, попробуй позже. Приносим извинения за неудобства.")
				response.Button("Выйти", "", true)
				return response
			}
		}

		if request.IsNewSession() {
			text += runSkillPhrases[rand.Intn(len(runSkillPhrases))]
			text += "\n"
		}

		if containsIgnoreCase(request.Text(), helpWords) {
			response.Text("Это твой личный гид в хроники коронавируса. Полезный навык, который помогает подготовиться на случай возможной эпидемии и быть всегда в курсе текущей ситуации. " +
				"\nВы можете спросить навык о статистике заболевания по регионам, узнать про очаги заражения, а также прослушать важные новости." +
				"\nМожешь спросить о симптомах коронавируса или о том, как от него защититься." +
				"\nВы можете оставить отзыв или предложение в каталоге навыков, либо написав мне в навыке \"Говорящая Почта\" на номер 1-3-2-6.")
			response.Button("Новости", "", true)
			response.Button("Статистика", "", true)
			response.Button("Очаги заражения", "", true)
			response.Button("Симптомы", "", true)
			response.Button("Как защититься", "", true)
			response.Button("Оценить навык", "https://dialogs.yandex.ru/store/skills/d5087c0d-hroniki-koronavirusa", false)
			response.Button("Написать на почту (1326)", "https://dialogs.yandex.ru/store/skills/eacbce8f-govoryashaya-po", false)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), yesterdayNews) {
			text += buildNews(currentStatus.Yesterday.AllNews)
			response.Text(text)
			response.Button("Актуальные новости", "", true)
			response.Button("Симптомы", "", true)
			response.Button("Как защититься", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), epicentrWords) {
			text += fmt.Sprintf(epicentr, c.printFire(currentStatus))
			response.Text(text)
			response.Button("Актуальные новости", "", true)
			response.Button("Симптомы", "", true)
			response.Button("Как защититься", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if strings.EqualFold(request.Text(), yesWord) || containsIgnoreCase(request.Text(), acceptNews) {
			text += buildNews(currentStatus.Current.AllNews)
			response.Text(text)
			response.Button("Вчерашние новости", "", true)
			response.Button("Статистика", "", true)
			response.Button("Очаги заражения", "", true)
			response.Button("Симптомы", "", true)
			response.Button("Как защититься", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), funWords) {
			text += "В мире объявлена пандемия коронавируса, полки магазинов пустеют, людям рекомендуют работать из дома..."
			response.Text(text)
			response.Button("Хроники коронавируса", "", true)
			response.Button("Очаги заражения", "", true)
			response.Button("Статистика", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), cancelWords) {
			text := endSkillPhrases[rand.Intn(len(endSkillPhrases))]
			response.Text(text + "\nСкажи - закончить, чтобы я отключился.")
			response.Button("Оценить навык", "https://dialogs.yandex.ru/store/skills/d5087c0d-hroniki-koronavirusa", false)
			response.Button("Закончить", "", false)
			return response
		}

		if containsIgnoreCase(request.Text(), symptomsWords) {
			text += symptomsPhrases[rand.Intn(len(symptomsPhrases))]
			response.Text(text)
			response.Button("Новости", "", true)
			response.Button("Статистика", "", true)
			response.Button("Очаги заражения", "", true)
			response.Button("Как защититься", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), protectWords) {
			text += howToProtectPhrases[rand.Intn(len(howToProtectPhrases))]
			response.Text(text)
			response.Button("Новости", "", true)
			response.Button("Статистика", "", true)
			response.Button("Симптомы", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), masksWords) {
			text += masksPhrases[rand.Intn(len(masksPhrases))]
			response.Text(text)
			response.Button("Новости", "", true)
			response.Button("Статистика", "", true)
			response.Button("Симптомы", "", true)
			response.Button("Как защититься", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if containsIgnoreCase(request.Text(), statsWords) {
			text += "Назовите или выберите страну или город, для которого хотите услышать статистику по заражениям"
			response.Text(text)
			response.Button("Россия", "", true)
			response.Button("Украина", "", true)
			response.Button("Беларусь", "", true)
			response.Button("Москва", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if len(request.Text()) > 3 && !containsIgnoreCase(request.Text(), runSkill) {
			curRegInfo := findRegion(currentStatus.Current.Countries, currentStatus.Current.Cities, request.Text())
			if curRegInfo != nil {
				yesterdayInfo := findRegion(currentStatus.Yesterday.Countries, currentStatus.Yesterday.Cities, request.Text())
				if yesterdayInfo != nil {
					confirmedTemplate := ""
					if curRegInfo.Confirmed-yesterdayInfo.Confirmed > 0 {
						confirmedTemplate = fmt.Sprintf(moreThanYesterday, curRegInfo.Confirmed-yesterdayInfo.Confirmed)
					}
					deathTemplate := ""
					if curRegInfo.Deaths-yesterdayInfo.Deaths > 0 {
						deathTemplate = fmt.Sprintf(moreThenDay, curRegInfo.Deaths-yesterdayInfo.Deaths)
					}
					curedTemplate := ""
					if curRegInfo.Cured-yesterdayInfo.Cured > 0 {
						curedTemplate = fmt.Sprintf(moreThanLastDay, curRegInfo.Cured-yesterdayInfo.Cured)
					}

					text += fmt.Sprintf(countryInfo, curRegInfo.Ru,
						curRegInfo.Confirmed, Plural(curRegInfo.Confirmed, "случай", "случая", "случаев"), confirmedTemplate,
						curRegInfo.Deaths, Plural(curRegInfo.Deaths, "человек", "человека", "человек"), deathTemplate,
						curRegInfo.Cured, Plural(curRegInfo.Cured, "человек", "человека", "человек"), curedTemplate)
				} else {
					text += fmt.Sprintf(countryInfoWithoutY, curRegInfo.Ru, curRegInfo.Confirmed, Plural(curRegInfo.Confirmed, "случай", "случая", "случаев"), curRegInfo.Deaths, Plural(curRegInfo.Deaths, "человек", "человека", "человек"), curRegInfo.Cured, Plural(curRegInfo.Cured, "человек", "человека", "человек"))
				}
				if curRegInfo.Ru == "Россия" {
					text += "\nСтатистика заражений по городам России: " + c.printFireCities(currentStatus)
				}
				response.Text(text)
			} else {
				text += fmt.Sprintf("Нет информации по региону \"%s\", попробуйте по другому.", request.Text())
				response.Text(text)
			}
			response.Button("Новости", "", true)
			response.Button("Статистика", "", true)
			response.Button("Очаги заражения", "", true)
			response.Button("Симптомы", "", true)
			response.Button("Как защититься", "", true)
			response.Button("Выйти", "", true)
			return response
		}

		if text == "" {
			text += runSkillPhrases[rand.Intn(len(runSkillPhrases))]
			text += "\n"
		}

		curRusReg := findRegion(currentStatus.Current.Countries, currentStatus.Current.Cities, "Россия")
		yesRusReg := findRegion(currentStatus.Yesterday.Countries, currentStatus.Yesterday.Cities, "Россия")
		confirmedTemplate := ""
		if currentStatus.Current.Confirmed-currentStatus.Yesterday.Confirmed > 0 {
			confirmedTemplate = fmt.Sprintf(moreThanYesterday, currentStatus.Current.Confirmed-currentStatus.Yesterday.Confirmed)
		}
		deathTemplate := ""
		if currentStatus.Current.Deaths-currentStatus.Yesterday.Deaths > 0 {
			deathTemplate = fmt.Sprintf(moreThenDay, currentStatus.Current.Deaths-currentStatus.Yesterday.Deaths)
		}
		rusConfirmedTemplate := ""
		if curRusReg.Confirmed-yesRusReg.Confirmed > 0 {
			rusConfirmedTemplate = fmt.Sprintf(moreThanYesterday, curRusReg.Confirmed-yesRusReg.Confirmed)
		}
		text += fmt.Sprintf(fullFirstPhrase,
			currentStatus.Current.Confirmed, Plural(currentStatus.Current.Confirmed, "случай", "случая", "случаев"), confirmedTemplate,
			currentStatus.Current.Deaths, Plural(currentStatus.Current.Deaths, "человек", "человека", "человек"), deathTemplate,
			currentStatus.Current.Cured, Plural(currentStatus.Current.Cured, "человек", "человека", "человек"),
			c.printFireNames(currentStatus),
			curRusReg.Confirmed, Plural(curRusReg.Confirmed, "человек", "человека", "человек"), rusConfirmedTemplate,
		)
		text += "\n"
		if user.Count == 1 {
			text += firstHi
			text += "\n"
		}
		text += newsPhrases[rand.Intn(len(newsPhrases))]
		response.Text(text)
		response.Button("Новости", "", true)
		response.Button("Статистика", "", true)
		response.Button("Очаги заражения", "", true)
		response.Button("Симптомы", "", true)
		response.Button("Как защититься", "", true)
		response.Button("Выйти", "", true)
		return response
	}
}

func (c Coronavirus) GetDayStatus() *DayStatus {
	status := &DayStatus{}
	err := c.connection.Collection("coronavirus").FindOne(bson.M{}, status)
	if err != nil {
		return nil
	}
	return status
}

func (c Coronavirus) GetUser(id string) *User {
	user := &User{}
	err := c.connection.Collection("users").FindOne(bson.M{"id": id}, user)
	if err != nil {
		return nil
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

func buildNews(news []New) string {
	strNew := ""
	for _, news_item := range news {
		strNew += "- " + news_item.Title + "\n"
	}
	return strNew
}

func findRegion(regions []Region, cities []Region, reg string) *Region {
	for _, region := range regions {
		if strings.EqualFold(region.Ru, reg) {
			return &region
		}
	}
	for _, region := range cities {
		if strings.EqualFold(region.Ru, reg) {
			return &region
		}
	}
	return nil
}

// Plural помогает согласовать слово с числительным.
func Plural(n int, singular, plural1, plural2 string) string {
	slice := strconv.Itoa(n)
	last := slice[len(slice)-1:]

	switch last {
	case "1":
		return singular
	case "2", "3", "4":
		return plural1
	default:
		return plural2
	}
}

func (c Coronavirus) saveUser(user *User) {
	err := c.connection.Collection("users").Save(user)
	if err != nil {
		log.Print("Error when saving to DB")
	}
}

func (c Coronavirus) printFireNames(dayStatus *DayStatus) string {
	strFire := make([]string, 0)
	for i := 0; i < 5; i++ {
		curInf := dayStatus.Current.Countries[i]
		strFire = append(strFire, curInf.Ru)
	}
	return strings.Join(strFire, ", ")
}

func (c Coronavirus) printFire(dayStatus *DayStatus) string {
	strFire := make([]string, 0)
	for i := 0; i < 20; i++ {
		curInf := dayStatus.Current.Countries[i]
		yesInf := findRegion(dayStatus.Yesterday.Countries, dayStatus.Yesterday.Cities, curInf.Ru)
		if yesInf == nil {
			yesInf = &curInf
		}
		str := fmt.Sprintf("%s - %d %s", curInf.Ru, curInf.Confirmed, Plural(curInf.Confirmed, "человек", "человека", "человек"))
		if curInf.Confirmed-yesInf.Confirmed > 0 {
			str += fmt.Sprintf(" (+%d за день)", curInf.Confirmed-yesInf.Confirmed)
		}
		strFire = append(strFire, str)
	}
	return strings.Join(strFire, "\n")
}

func (c Coronavirus) printFireCities(dayStatus *DayStatus) string {
	strFire := make([]string, 0)
	for _, city := range dayStatus.Current.Cities {
		yesInf := findRegion(dayStatus.Yesterday.Countries, dayStatus.Yesterday.Cities, city.Ru)
		if yesInf == nil {
			yesInf = &city
		}
		str := fmt.Sprintf("%s - %d %s", city.Ru, city.Confirmed, Plural(city.Confirmed, "человек", "человека", "человек"))
		if city.Confirmed-yesInf.Confirmed > 0 {
			str += fmt.Sprintf(" (+%d за день)", city.Confirmed-yesInf.Confirmed)
		}
		strFire = append(strFire, str)
	}
	return strings.Join(strFire, ", ")
}

func (c Coronavirus) grabData() *DayStatus {
	currentStatus := c.GetDayStatus()
	resp, err := c.httpClient.Get(coronavirusApi)
	if err != nil {
		log.Print("Error: when getting coronavirus response")
		return currentStatus
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print("Error: when reading coronavirus response")
		return currentStatus
	}

	bodyString := string(bodyBytes)
	bodyString = strings.Replace(bodyString, "    window.dataFromServer = ", "", 1)
	bodyBytes = []byte(bodyString)

	var result Response
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		log.Print("Error: when unmarhsal coronavirus response")
		return currentStatus
	}

	cities := result.Cities.Data.Cities
	regions := result.Countries.Data.Countries

	confirmed := 0
	deaths := 0
	cured := 0
	for _, info := range regions {
		if info.IsCountry {
			confirmed += info.Confirmed
			deaths += info.Deaths
			cured += info.Cured
		}
	}

	allNews := result.AllNews.Data.AllNews
	importantNews := result.ImportantNews.Data.ImportantNews

	currentInfo := CoronavirusInfo{
		Countries:     regions,
		Cities:        cities,
		Confirmed:     confirmed,
		Deaths:        deaths,
		Cured:         cured,
		ImportantNews: importantNews,
		AllNews:       allNews,
	}

	if currentStatus == nil {
		currentStatus = &DayStatus{Current: currentInfo, Yesterday: currentInfo}
	} else {
		currentStatus.Current = currentInfo
		if currentStatus.Modified.Day() != time.Now().Day() {
			currentStatus.Yesterday = currentInfo
		}
	}

	err = c.connection.Collection("coronavirus").Save(currentStatus)
	if err != nil {
		log.Print("Error when saving to DB")
	}
	log.Print("New info saved")
	return currentStatus
}
