package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	_ "embed"
)

type Client struct {
	conn     *websocket.Conn
	nickname string
	room     *Room
}

type Room struct {
	clients   map[*Client]bool
	broadcast chan Message
	join      chan *Client
	leave     chan *Client
}

type Message struct {
	Sender string `json:"sender"`
	Text   string `json:"text"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

//go:embed static/index.html
var indexHTML string

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(indexHTML))
	})

	room := NewRoom()
	go room.run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}

		defer conn.Close()

		fmt.Println("Client connected")

		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		nickname := string(p)

		client := &Client{
			conn:     conn,
			room:     room,
			nickname: nickname,
		}

		client.room.join <- client

		go client.write()
		client.read()
	})

	fmt.Println("Listening on port 9000")
	log.Fatal(http.ListenAndServe(":9000", nil))
}

func NewRoom() *Room {
	return &Room{
		clients:   make(map[*Client]bool),
		broadcast: make(chan Message),
		join:      make(chan *Client),
		leave:     make(chan *Client),
	}
}

func (r *Room) run() {
	for {
		select {
		case client := <-r.join:
			r.clients[client] = true
		case client := <-r.leave:
			delete(r.clients, client)
		case msg := <-r.broadcast:
			for client := range r.clients {
				client.conn.WriteJSON(msg)
			}
			fmt.Println(msg)

		}
	}
}

func (c *Client) read() {
	defer func() {
		c.room.leave <- c
		c.conn.Close()
	}()

	for {
		_, p, err := c.conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Println(string(p))

		c.room.broadcast <- Message{
			Sender: c.nickname,
			Text:   string(p),
		}
	}
}

func (c *Client) write() {
	defer func() {
		c.conn.Close()
	}()
	for msg := range c.room.broadcast {
		if err := c.conn.WriteJSON(msg); err != nil {
			log.Println(err)
			return
		}
	}
}
