package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type message struct {
	Action  string `json:"action"`
	Room    string `json:"room"`
	Content string `json:"content"`
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	pingPeriod = 10 * time.Second
	pongWait   = 15 * time.Second
)

var rooms = make(map[string][]*websocket.Conn)
var mutex sync.Mutex

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error while connection: ", err)
		return
	}

	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for range ticker.C {
			err := conn.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				log.Println("Ping failed, closing connection:", err)
				conn.Close()
				return
			}
		}
	}()

	var currentRoom string

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("error while reading message: ", err)
			break
		}

		var message message
		if err := json.Unmarshal(msg, &message); err != nil {
			log.Println("error while unmarshaling: ", err)
			continue
		}

		switch message.Action {
		case "join":
			mutex.Lock()
			rooms[message.Room] = append(rooms[message.Room], conn)
			mutex.Unlock()
			currentRoom = message.Room
			log.Println("Client join Room: ", message.Room)
			broadcast(message.Room, "Someone joined "+message.Room)
		case "leave":
			mutex.Lock()
			remove(message.Room, conn)
			mutex.Unlock()
			log.Println("Client leave Room: ", message.Room)
			broadcast(message.Room, "Someone left the room: "+message.Room)
			currentRoom = ""

		case "message":
			broadcast(message.Room, message.Content)
		}
	}

	if currentRoom != "" {
		mutex.Lock()
		remove(currentRoom, conn)
		mutex.Unlock()
		broadcast(currentRoom, "Someone disconnected")
	}
}

func broadcast(room string, msg string) {
	mutex.Lock()
	defer mutex.Unlock()

	for _, c := range rooms[room] {
		err := c.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			log.Println("Error while Broadcasting message: ", err)
			return
		}
	}
}

func remove(room string, conn *websocket.Conn) {
	conns := rooms[room]

	for i, c := range conns {
		if c == conn {
			rooms[room] = append(conns[:i], conns[i+1:]...)
			break
		}
	}

	if len(rooms[room]) == 0 {
		delete(rooms, room)
	}
}

func main() {

	r := gin.Default()

	r.GET("/ws", func(c *gin.Context) {
		wsHandler(c.Writer, c.Request)
	})

	r.Run(":8080")

}
