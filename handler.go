package main

import (
	"encoding/json"
	"github.com/azzzak/alice"
	"log"
	"net/http"
	"sync"
)

var statistics = map[string]map[string]int{}

func Handler() func(func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return func(f func(w http.ResponseWriter, r *http.Request)) http.Handler {
		h := http.HandlerFunc(f)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		})
	}
}

func JsonContentType(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		h.ServeHTTP(w, r)
	})
}

func handleRequest(f func(request *alice.Request, response *alice.Response) *alice.Response) func(w http.ResponseWriter, r *http.Request) {
	reqPool := sync.Pool{
		New: func() interface{} {
			return new(alice.Request)
		},
	}

	respPool := sync.Pool{
		New: func() interface{} {
			return new(alice.Response)
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(200)
			w.Write([]byte("Ok"))
			return
		}
		req := reqPool.Get().(*alice.Request)
		defer reqPool.Put(req)

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(req); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resp := initResponse(respPool, req)

		if req.Request.OriginalUtterance != "ping" {
			resp = f(req, resp)
			if stats, ok := statistics[r.RequestURI]; ok {
				if _, ok := stats[req.Session.UserID]; ok {
					stats[req.Session.UserID]++
				} else {
					stats[req.Session.UserID] = 1
					stats["totalUsers"]++
				}
				stats["totalMessages"]++
			} else {
				statistics[r.RequestURI] = map[string]int{}
				statistics[r.RequestURI][req.Session.UserID] = 1
				statistics[r.RequestURI]["totalMessages"] = 1
				statistics[r.RequestURI]["totalUsers"] = 1
			}
		} else {
			resp.Text("4 пакета отправлено, 3 пакета получено. 1 пакет украли на почте")
			log.Print("ping request")
		}

		log.Printf("Incomming request from host: %s. Request: { userId: %s, text: %s}. Response: { text: %s}",
			r.RemoteAddr, req.Session.UserID, req.Text(), resp.Response.Text)
		b, err := json.Marshal(resp)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(200)
		w.Write(b)
	}
}

func handleHealthRequest(dialogs []Dialog) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		for _, v := range dialogs {
			ok, message := v.Health()
			if !ok {
				w.WriteHeader(500)
				w.Write([]byte(message))
				return
			}
		}
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}
}

func handleStatisticsRequest() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		responseStatistics := map[string]map[string]int{}

		for k, v := range statistics {
			responseStatistics[k] = map[string]int{}
			responseStatistics[k]["totalMessages"] = v["totalMessages"]
			responseStatistics[k]["totalUsers"] = v["totalUsers"]
		}

		b, err := json.Marshal(responseStatistics)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(200)
		w.Write(b)
	}
}

func initResponse(respPool sync.Pool, req *alice.Request) *alice.Response {
	resp := respPool.Get().(*alice.Response)
	resp.Session.MessageID = req.Session.MessageID
	resp.Session.SessionID = req.Session.SessionID
	resp.Session.UserID = req.Session.UserID
	resp.Version = "1.0"
	return resp
}
