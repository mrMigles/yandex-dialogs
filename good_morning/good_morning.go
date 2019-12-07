package good_morning

import (
	"github.com/azzzak/alice"
)

type GoodMorning struct{}

func (GoodMorning) HandleRequest() func(request *alice.Request, response *alice.Response) *alice.Response {
	panic("implement me")
}

func (GoodMorning) GetPath() string {
	panic("implement me")
}

func (GoodMorning) Health() (result bool, message string) {
	panic("implement me")
}
