// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

type SendMsg struct {
	Message  string  `json:"message"`
	From     string  `json:"from"`
	Sendtime string  `json:"sendtime"`
	UserID   float64 `json:"user_id"`
}

type ReceiveMsg struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	db, err := sql.Open("mysql", "root:dasomDASOM@tcp(db.dasom.io:3306)/dasomweb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		receive := ReceiveMsg{}
		json.Unmarshal([]byte(message), &receive)

		if receive.Type == "message" {
			temp := receive.Data.(map[string]interface{})
			send := SendMsg{
				Message:  strings.Replace(temp["message"].(string), "\n", " ", -1),
				From:     temp["from"].(string),
				Sendtime: temp["sendtime"].(string),
				UserID:   temp["user_id"].(float64),
			}
			fmt.Println(send.Message)
			//send = receive.Data.(map[string]interface{})
			//json.Unmarshal(map[string]receive.Data, &send)

			_, err = db.Exec("INSERT chat (message, sendtime, user_id) values (?, ?, ?)", send.Message, send.Sendtime, send.UserID)
			if err != nil {
				log.Fatal(err)
			}

			message, _ = json.Marshal(send)
			message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
			c.hub.broadcast <- message

		} else if receive.Type == "command" {
			rows, _ := db.Query("SELECT chat.message, users.name, chat.sendtime, chat.user_id FROM dasomweb.chat, dasomweb.users where chat.user_id = users.id order by chat.id desc limit ?, 10", receive.Data.(map[string]interface{})["idx"])
			for rows.Next() {
				send := SendMsg{}
				err = rows.Scan(&send.Message, &send.From, &send.Sendtime, &send.UserID)
				message, _ = json.Marshal(send)
				message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
				c.hub.broadcast <- message
				fmt.Println(message)
			}
		}

	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 512)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
