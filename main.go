// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
)

var addr = flag.String("addr", ":8082", "http service address")

type HostSwitch map[string]http.Handler

func (hs HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler := hs[r.Host]; handler != nil {
		ip := r.RemoteAddr
		log.Println("[Req]", r.Method, r.URL, ip)
		handler.ServeHTTP(w, r)
	} else {
		http.Error(w, "Forbidden hostname : "+r.Host, http.StatusForbidden)
		return
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home.html")
}

func main() {
	flag.Parse()

	// db 설정

	hub := newHub()
	go hub.run()

	router := httprouter.New()
	router.HandlerFunc("GET", "/", serveHome)
	router.HandlerFunc("GET", "/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	handler := cors.AllowAll().Handler(router)
	hs := make(HostSwitch)
	hs["chat-api"] = handler
	hs["localhost"+*addr] = handler

	err := http.ListenAndServe(*addr, handler)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}
