package main

import (
	"encoding/json"
	"github.com/azzzak/alice"
	"log"
	"net/http"
	"sync"
)

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

		resp = f(req, resp)

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

func initResponse(respPool sync.Pool, req *alice.Request) *alice.Response {
	resp := respPool.Get().(*alice.Response)
	resp.Session.MessageID = req.Session.MessageID
	resp.Session.SessionID = req.Session.SessionID
	resp.Session.UserID = req.Session.UserID
	resp.Version = "1.0"
	return resp
}
