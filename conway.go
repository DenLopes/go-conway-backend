package main

import (
	"math/rand"
	"sync"
	"time"
)

const (
	GRID_SIZE    = 1024
	ALIVE_CHANCE = 20
	TICK_RATE    = 96 * time.Millisecond
	GRID_BYTES   = (GRID_SIZE*GRID_SIZE + 7) / 8
	workerCount  = 4
)

var (
	grid            = make(Grid, GRID_BYTES)
	interactionGrid = make(Grid, GRID_BYTES)
	// The code in the masks are the binary representation of the numbers 128, 64, 32, 16, 8, 4, 2, 1
	// 128 = 10000000
	// 64  = 01000000
	// 32  = 00100000
	// 16  = 00010000
	// 8   = 00001000
	// 4   = 00000100
	// 2   = 00000010
	// 1   = 00000001
	bitMasks = [8]byte{0x80, 0x40, 0x20, 0x10, 0x08, 0x04, 0x02, 0x01}
)

// Helper function to wrap coordinates
func wrapCoordinate(coord int) int {
	// Using modulo to wrap around
	// Adding GRID_SIZE before modulo to handle negative numbers
	// Example: -1 % 10 = -1, but (-1 + 10) % 10 = 9
	return ((coord % GRID_SIZE) + GRID_SIZE) % GRID_SIZE
}

// Convert 2D coordinates to 1D index with wrapping
// We use this to convert x, y coordinates to a single index in the grid, as the grid is a 1D slice
func getWrappedIndex(x, y int) int {
	wrappedX := wrapCoordinate(x)
	wrappedY := wrapCoordinate(y)
	return wrappedY*GRID_SIZE + wrappedX
}

func getBit(grid Grid, i int) bool {
	byteIndex := i >> 3
	bitIndex := i & 0x07
	// Think in binary for this one
	//     When i = 0:
	// 1 << 0 = 00000001 (mask for position 0)
	// byte   = 00000110
	// & (AND)= 00000000 (result = 0, meaning bit 0 is NOT set)

	// When i = 1:
	// 1 << 1 = 00000010 (mask for position 1)
	// byte   = 00000110
	// & (AND)= 00000010 (result > 0, meaning bit 1 is SET)

	// When i = 2:
	// 1 << 2 = 00000100 (mask for position 2)
	// byte   = 00000110
	// & (AND)= 00000100 (result > 0, meaning bit 2 is SET)

	// When i = 3:
	// 1 << 3 = 00001000 (mask for position 3)
	// byte   = 00000110
	// & (AND)= 00000000 (result = 0, meaning bit 3 is NOT set)
	// ... and so on
	return (grid[byteIndex] & bitMasks[bitIndex]) != 0
}

func setBit(grid *Grid, i int, value bool) {
	byteIndex := i >> 3
	bitIndex := i & 0x07
	mask := bitMasks[bitIndex]
	if value {
		// Set the bit
		// Example: 00000000 | 00000010 = 00000010
		// The position of the 1 in the mask is the bit we want to set
		// The OR operator will set the bit to 1 without changing the other bits
		(*grid)[byteIndex] |= mask
	} else {
		// Clear the bit
		// Example: 00000010 &^ 00000010 = 00000000
		// The position of the 1 in the mask is the bit we want to clear
		// The AND NOT operator will clear the bit to 0 without changing the other bits
		(*grid)[byteIndex] &^= mask
	}
}

var gridPool = sync.Pool{
	New: func() interface{} {
		grid := make(Grid, GRID_BYTES)
		return &grid
	},
}

func processSegment(start, end int, newGrid *Grid, wg *sync.WaitGroup, mu *sync.Mutex) {
	defer wg.Done()

	tempGridPtr := gridPool.Get().(*Grid)
	tempGrid := *tempGridPtr
	for i := range tempGrid {
		tempGrid[i] = 0
	}
	defer gridPool.Put(tempGridPtr)

	for y := start; y < end; y++ {
		currentY := y / GRID_SIZE
		currentX := y % GRID_SIZE

		aliveNeighbors := 0

		// Check all 8 neighbors with wrapping
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}

				neighborIndex := getWrappedIndex(currentX+dx, currentY+dy)
				if getBit(grid, neighborIndex) {
					aliveNeighbors++
					if aliveNeighbors > 3 {
						break
					}
				}
			}
			if aliveNeighbors > 3 {
				break
			}
		}

		currentCell := getBit(grid, y)
		shouldLive := (currentCell && aliveNeighbors == 2) || aliveNeighbors == 3
		if shouldLive {
			setBit(&tempGrid, y, true)
		}
	}

	mu.Lock()
	for i := 0; i < len(tempGrid); i++ {
		(*newGrid)[i] |= tempGrid[i]
	}
	mu.Unlock()
}

func NewGeneration() Grid {
	newGrid := interactionGrid
	interactionGrid = make(Grid, GRID_BYTES)

	var wg sync.WaitGroup
	var mu sync.Mutex

	totalCells := GRID_SIZE * GRID_SIZE
	segmentSize := totalCells / workerCount

	for i := 0; i < workerCount; i++ {
		start := i * segmentSize
		end := start + segmentSize
		if i == workerCount-1 {
			end = totalCells
		}

		wg.Add(1)
		go processSegment(start, end, &newGrid, &wg, &mu)
	}

	wg.Wait()
	grid = newGrid
	return newGrid
}

func GameLoop() {
	ticker := time.NewTicker(TICK_RATE)
	defer ticker.Stop()

	for range ticker.C {
		if len(Clients) > 0 {
			Broadcast <- NewGeneration()
			continue
		}
		Broadcast <- grid
	}
}

func ClearGrid() {
	grid = make(Grid, GRID_BYTES)
	interactionGrid = make(Grid, GRID_BYTES)
}

func AddPointToGrid(p Point) {
	wrappedX := wrapCoordinate(p.X)
	wrappedY := wrapCoordinate(p.Y)
	setBit(&interactionGrid, wrappedY*GRID_SIZE+wrappedX, true)
}

func RandGrid() error {
	for y := 0; y < GRID_SIZE; y++ {
		for x := 0; x < GRID_SIZE; x++ {
			if rand.Intn(100) < ALIVE_CHANCE {
				setBit(&interactionGrid, y*GRID_SIZE+x, true)
			}
		}
	}
	return nil
}
