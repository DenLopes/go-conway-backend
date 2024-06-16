package main

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// type Grid [601][261]bool

type Grid [300][100]bool

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type GameState struct {
	Width   int  `json:"width"`
	Height  int  `json:"height"`
	Playing bool `json:"playing"`
}

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

var (
	grid      Grid
	gameSpeed time.Duration = 120
	gameState bool          = true
)

func NewGeneration() []Point {
	var newGrid Grid
	aliveCells := []Point{}
	var mutex sync.Mutex
	var workerGroup sync.WaitGroup

	taskChan := make(chan Point)

	worker := func() {
		defer workerGroup.Done()
		for task := range taskChan {
			i, j := task.X, task.Y
			aliveNeighbors := countAliveNeighbors(grid, i, j)
			if grid[i][j] {
				if aliveNeighbors == 2 || aliveNeighbors == 3 {
					newGrid[i][j] = true
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

	const numWorkers = 3
	workerGroup.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		go worker()
	}

	for i := 1; i < len(grid)-1; i++ {
		for j := 1; j < len(grid[0])-1; j++ {
			taskChan <- Point{i, j}
		}
	}

	close(taskChan)
	workerGroup.Wait()

	grid = newGrid
	return aliveCells
}

func countAliveNeighbors(grid Grid, i, j int) int {
	aliveNeighbors := 0
	for _, neighbor := range neighbors {
		ni, nj := i+neighbor.X, j+neighbor.Y
		if grid[ni][nj] {
			aliveNeighbors++
		}
	}
	return aliveNeighbors
}

func GetGameState() GameState {
	return GameState{
		Height:  len(grid[0]) - 1,
		Width:   len(grid) - 1,
		Playing: gameState,
	}
}

var currentCells []Point

func GameLoop() {
	ticker := time.NewTicker(gameSpeed * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if gameState {
			cellsTobeDrawn := NewGeneration()
			currentCells = cellsTobeDrawn
			Broadcast <- cellsTobeDrawn
		} else {
			Broadcast <- currentCells
		}
	}
}

func StartGame() {
	gameState = true
}

func StopGame() {
	gameState = false
}

func ClearGrid() {
	grid = Grid{}
	currentCells = []Point{}
}

func RandGrid() error {
	currentCells = []Point{}
	if len(grid) == 0 {
		return fmt.Errorf("grid is empty")
	}
	for i := 1; i < len(grid)-1; i++ {
		for j := 1; j < len(grid[0])-1; j++ {
			isAlive := math.Round(rand.Float64()) > 0.5
			grid[i][j] = isAlive
			if isAlive {
				currentCells = append(currentCells, Point{i, j})
			}
		}
	}
	return nil
}

func AddPointToGrid(p Point) {
	grid[p.X][p.Y] = true
}
