package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/golang/snappy"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

var (
	Clients    = make(map[*websocket.Conn]bool)
	clientsMux sync.Mutex
	Broadcast  = make(chan Grid)
)

func main() {
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, World!")
	})

	server := &http.Server{Addr: ":8080"}

	go func() {
		fmt.Println("Server is running on port 8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on :8080: %v\n", err)
		}
	}()

	go handleBroadcast()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go GameLoop()

	<-stop

	fmt.Println("Shutting down server...")
	if err := server.Close(); err != nil {
		log.Fatalf("Server Close: %v", err)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading websocket:", err)
		return
	}
	defer ws.Close()

	clientsMux.Lock()
	Clients[ws] = true
	clientsMux.Unlock()

	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			clientsMux.Lock()
			delete(Clients, ws)
			clientsMux.Unlock()
			log.Println("Error reading message from client:", err)
			break
		}
		switch string(p) {
		case "random":
			fmt.Println("Generating random grid")
			err := RandGrid()
			if err != nil {
				log.Println("Error generating random grid:", err)
				continue
			}
		case "clear":
			fmt.Println("Clearing grid")
			ClearGrid()
		default:
			var point Point
			err := json.Unmarshal(p, &point)
			if err != nil {
				log.Println("Error unmarshaling point:", err)
				continue
			}
			fmt.Println("Adding point to the grid")
			AddPointToGrid(point)
		}
	}
}

func handleBroadcast() {
	for {
		grid := <-Broadcast
		compressedGrid := compress(grid[:])
		clientsMux.Lock()
		for client := range Clients {
			err := client.WriteMessage(websocket.BinaryMessage, compressedGrid)
			if err != nil {
				log.Println("Error writing message to client:", err)
				client.Close()
				delete(Clients, client)
			}
		}
		clientsMux.Unlock()
	}
}

func compress(data []byte) []byte {
	return snappy.Encode(nil, data)
}
