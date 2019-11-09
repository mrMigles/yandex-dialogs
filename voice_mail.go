package main

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

type VoiceMail struct {
}

func (handler VoiceMail) handleRequest() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				w.WriteHeader(500)
				w.Write([]byte(fmt.Sprintf("{\"Error\": \"%+v\"}", r)))
				fmt.Println("recovered from ", r)
				debug.PrintStack()
			}
		}()
		w.WriteHeader(200)
		w.Write([]byte("Ok"))
	}
}
