package main

import (
	"math/rand"
	"sync"
	"time"
)

// the grid is 255x255 but we are using 256x256 to avoid out of bounds errors
type Grid [8192]byte

var (
	grid      Grid
	neighbors = []int{
		-257, -256, -255,
		-1, 1,
		255, 256, 257,
	}
	interactionGrid Grid
	workerCount     = 2 // Number of concurrent workers
)

func getBit(grid Grid, i int) bool {
	byteIndex := i / 8
	bitIndex := 7 - (i % 8) // Adjust bit index to reverse bit order
	return (grid[byteIndex] & (1 << bitIndex)) != 0
}

func setBit(grid *Grid, i int, value bool) {
	byteIndex := i / 8
	bitIndex := 7 - (i % 8) // Adjust bit index to reverse bit order
	if value {
		grid[byteIndex] |= (1 << bitIndex)
	} else {
		grid[byteIndex] &^= (1 << bitIndex)
	}
}

func processSegment(start, end int, newGrid *Grid, wg *sync.WaitGroup, mu *sync.Mutex) {
	defer wg.Done()

	tempGrid := Grid{}
	for y := start; y < end; y++ {
		if y%256 == 0 || y%256 == 255 {
			continue
		}
		aliveNeighbors := 0
		for _, n := range neighbors {
			if getBit(grid, y+n) {
				aliveNeighbors++
			}
		}
		if getBit(grid, y) {
			if aliveNeighbors == 2 || aliveNeighbors == 3 {
				setBit(&tempGrid, y, true)
			}
		} else {
			if aliveNeighbors == 3 {
				setBit(&tempGrid, y, true)
			}
		}
	}

	// Merge tempGrid into newGrid
	mu.Lock()
	for i := 0; i < len(tempGrid); i++ {
		newGrid[i] |= tempGrid[i]
	}
	mu.Unlock()
}

func NewGeneration() Grid {
	newGrid := interactionGrid
	interactionGrid = Grid{}

	var wg sync.WaitGroup
	var mu sync.Mutex

	segmentSize := (len(grid)*8 - 512) / workerCount
	for i := 0; i < workerCount; i++ {
		start := 256 + i*segmentSize
		end := start + segmentSize
		if i == workerCount-1 {
			end = (len(grid) * 8) - 256
		}
		wg.Add(1)
		go processSegment(start, end, &newGrid, &wg, &mu)
	}

	wg.Wait()

	grid = newGrid
	return newGrid
}

func GameLoop() {
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if len(Clients) > 0 {
			cellsTobeDrawn := NewGeneration()
			Broadcast <- cellsTobeDrawn
		} else {
			Broadcast <- grid
		}
	}
}

func ClearGrid() {
	interactionGrid = Grid{}
	grid = Grid{}
}

func AddPointToGrid(p Point) {
	i := p.Y*256 + p.X
	setBit(&interactionGrid, i, true)
}

func RandGrid() error {
	for i := 256; i < (len(interactionGrid)*8)-256; i++ {
		if i%256 == 0 || i%256 == 255 {
			continue
		}
		if rand.Intn(100) < 20 {
			setBit(&interactionGrid, i, true)
		}
	}
	return nil
}
