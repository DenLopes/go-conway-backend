package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type GridSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Grid [601][601]bool

var neighbors = []Point{
	{-1, -1},
	{-1, 0},
	{-1, 1},
	{0, -1},
	{0, 1},
	{1, -1},
	{1, 0},
	{1, 1},
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	clients    = make(map[*websocket.Conn]bool)
	clientsMux sync.Mutex
	broadcast  = make(chan []Point)
	grid       Grid
)

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

	// Send grid dimensions to client on connection
	gridDimensions := GridSize{Width: len(grid[0]) - 1, Height: len(grid) - 1}
	dimensionsMessage, err := json.Marshal(gridDimensions)
	if err != nil {
		log.Println("Error marshaling grid dimensions:", err)
	} else {
		if err := ws.WriteMessage(websocket.TextMessage, dimensionsMessage); err != nil {
			log.Println("Error sending grid dimensions to client:", err)
			return
		}
	}

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			clientsMux.Lock()
			delete(clients, ws)
			clientsMux.Unlock()
			log.Println("Error reading message from client:", err)
			break
		}
	}
}

func main() {
	err := initGrid()
	if err != nil {
		log.Fatal(err)
	}
	err = randGrid()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/ws", wsHandler)
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

	go gameLoop()

	<-stop

	fmt.Println("Shutting down server...")
	if err := server.Close(); err != nil {
		log.Fatalf("Server Close: %v", err)
	}
}

func gameLoop() {
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		cellsTobeDrawn := newGeneration()
		broadcast <- cellsTobeDrawn
	}
}

func initGrid() error {
	if len(grid) == 0 {
		return fmt.Errorf("grid is empty")
	}
	for i := 0; i < len(grid); i++ {
		for j := 0; j < len(grid[0]); j++ {
			grid[i][j] = false
		}
	}
	return nil
}

func randGrid() error {
	if len(grid) == 0 {
		return fmt.Errorf("grid is empty")
	}
	for i := 1; i < len(grid)-1; i++ {
		for j := 1; j < len(grid[0])-1; j++ {
			grid[i][j] = math.Round(rand.Float64()) > 0.5
		}
	}
	return nil
}

func newGeneration() []Point {
	newGrid := grid
	aliveCells := []Point{}
	var mutex sync.Mutex
	var workerGroup sync.WaitGroup

	type cellTask struct {
		i, j int
	}

	taskChan := make(chan cellTask)

	worker := func() {
		defer workerGroup.Done()
		for task := range taskChan {
			i, j := task.i, task.j
			aliveNeighbors := countAliveNeighbors(grid, i, j)
			if grid[i][j] {
				if aliveNeighbors < 2 || aliveNeighbors > 3 {
					newGrid[i][j] = false
				} else {
					mutex.Lock()
					aliveCells = append(aliveCells, Point{i, j})
					mutex.Unlock()
				}
			} else {
				if aliveNeighbors == 3 {
					newGrid[i][j] = true
					mutex.Lock()
					aliveCells = append(aliveCells, Point{i, j})
					mutex.Unlock()
				}
			}
		}
	}

	const numWorkers = 4
	workerGroup.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		go worker()
	}

	for i := 1; i < len(grid)-1; i++ {
		for j := 1; j < len(grid[0])-1; j++ {
			taskChan <- cellTask{i, j}
		}
	}

	close(taskChan)
	workerGroup.Wait()

	grid = newGrid
	return aliveCells
}

func handleBroadcast() {
	for {
		cells := <-broadcast
		message, err := json.Marshal(cells)
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

func countAliveNeighbors(grid Grid, i, j int) int {
	aliveNeighbors := 0
	for _, neighbor := range neighbors {
		ni, nj := i+neighbor.X, j+neighbor.Y
		if ni >= 0 && ni < len(grid) && nj >= 0 && nj < len(grid[0]) && grid[ni][nj] {
			aliveNeighbors++
		}
	}
	return aliveNeighbors
}
