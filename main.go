package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/text/cases"
)

type message struct {
	Action  string `json:"action"`
	Room    string `json:"room"`
	Content string `json:"content"`
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var rooms = make(map[string][]**websocket.Conn)
var mutex sync.Mutex

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w,r,nil)
	if err != nil {
		log.Println("Error while connection", err)
		return
	}

	defer conn.Close()

	var currentRoom string

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error while reading message: ", err)
		}

		var m message

		if err := json.Unmarshal(msg, &m); err != nil {
			log.Println("Error while unmarshal: ", err)
			continue
		}

		switch m.Action {
		case "join":
			mutex.Lock()
			rooms[m.Room] = append(rooms[m.Room], &conn)
			mutex.Unlock()
			currentRoom = m.Room
			log.Println("client joined Room: ", m.Room)
			broadcast(m.Room, "Someone Joined" + m.Room)

		case "leave":
			mutex.Lock()
			removeConn(m.Room, conn)
			mutex.Unlock()
			broadcast(m.Room, "someone left" + m.Room)
			log.Println("client leave room", m.Room)
			currentRoom = ""

		case "message":
			broadcast(m.Room, m.Content)
		}



		if currentRoom != "" {
			mutex.Lock()
			removeConn(m.Room, conn)
			mutex.Unlock()
			broadcast(currentRoom, "someone disconnected")
		}
	}



	
}