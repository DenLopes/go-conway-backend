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

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Packet struct {
	GameState GameState `json:"gameState"`
	Cells     []Point   `json:"cells"`
}

var (
	clients    = make(map[*websocket.Conn]bool)
	clientsMux sync.Mutex
	Broadcast  = make(chan []Point)
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
	clients[ws] = true
	clientsMux.Unlock()

	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			clientsMux.Lock()
			delete(clients, ws)
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
		case "start":
			fmt.Println("Starting game")
			StartGame()
		case "stop":
			fmt.Println("Stopping game")
			StopGame()
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
		cells := <-Broadcast
		gameState := GetGameState()
		message, err := json.Marshal(Packet{GameState: gameState, Cells: cells})
		if err != nil {
			log.Println("Error marshaling cells:", err)
			continue
		}

		clientsMux.Lock()
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Println("Error writing message to client:", err)
				client.Close()
				delete(clients, client)
			}
		}
		clientsMux.Unlock()
	}
}
