package simulation

import (
	"context"
	"math"
	"math/rand"
	"runtime"
	"sync"
)

// Engine encapsule l'état complet et le moteur de calcul d'une simulation ZombieHorde.
type Engine struct {
	rng              *rand.Rand
	obstacles        []Obstacle
	horde            []Zombie
	exits            []ExitZone
	newZombies       []Zombie
	survivors        []Survivor
	heatmap          [GridCols * GridRows]int
	survPoints       []Point
	escaped          int
	centerRoomX      float64
	centerRoomY      float64
	centerRoomSize   float64
	doorNorthX       float64
	doorNorthY       float64
	doorSouthX       float64
	doorSouthY       float64
	grid             [GridCols * GridRows]Cell
	survivorNext     [TotalSurvivors]int32
	zombieNext       [TotalSurvivors]int32
	wasAlerted       [TotalSurvivors]bool
	posSnapshotX     [TotalSurvivors]float64
	posSnapshotY     [TotalSurvivors]float64
	workerCount      int
	chunks           []SurvivorChunk
	workerRng        []*rand.Rand
	workerEscaped    []int
	workerExitsArmed [][4]bool
	workerStartCh    []chan struct{}
	workerDoneWg     sync.WaitGroup
	workerCtx        context.Context
	workerCancel     context.CancelFunc
	closed           bool
}

// NewEngineWithWorkers instancie une nouvelle simulation avec un pool de workers borné
// pour découper équitablement la population de survivants.
func NewEngineWithWorkers(seed int64, numWorkers int) *Engine {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	if numWorkers > TotalSurvivors {
		numWorkers = TotalSurvivors
	}

	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		rng:              rand.New(rand.NewSource(seed)),
		escaped:          0,
		obstacles:        make([]Obstacle, 0, 6+ObstacleCount),
		newZombies:       make([]Zombie, 0, TotalSurvivors),
		survPoints:       make([]Point, 0, TotalSurvivors),
		workerCount:      numWorkers,
		workerCtx:        ctx,
		workerCancel:     cancel,
		workerRng:        make([]*rand.Rand, numWorkers),
		workerEscaped:    make([]int, numWorkers),
		workerExitsArmed: make([][4]bool, numWorkers),
		workerStartCh:    make([]chan struct{}, numWorkers),
	}

	for i := 0; i < numWorkers; i++ {
		e.workerRng[i] = rand.New(rand.NewSource(seed + int64(i)*10007 + 1))
		e.workerStartCh[i] = make(chan struct{}, 1)
	}

	e.initEnvironment()
	e.initSurvivors()
	e.initHorde()
	e.initSurvivorChunks(numWorkers)
	e.startWorkerPool()

	runtime.SetFinalizer(e, func(eng *Engine) {
		eng.Close()
	})

	return e
}

// NewEngine instancie et initialise une nouvelle simulation déterministe
// bornée sur runtime.NumCPU() cœurs.
func NewEngine(seed int64) *Engine {
	return NewEngineWithWorkers(seed, runtime.NumCPU())
}

// WorkerCount retourne le nombre de workers alloués au pool.
func (e *Engine) WorkerCount() int {
	return e.workerCount
}

// Close arrête proprement les goroutines persistantes du Worker Pool.
func (e *Engine) Close() {
	if e.closed {
		return
	}
	e.closed = true
	if e.workerCancel != nil {
		e.workerCancel()
	}
}

// startWorkerPool lance les goroutines persistantes bornées sur runtime.NumCPU().
func (e *Engine) startWorkerPool() {
	for i := 0; i < e.workerCount; i++ {
		workerID := i
		go e.workerLoop(workerID)
	}
}

func (e *Engine) workerLoop(workerID int) {
	chunk := e.chunks[workerID]
	for {
		select {
		case <-e.workerCtx.Done():
			return
		case <-e.workerStartCh[workerID]:
			e.updateSurvivorAlertsChunk(workerID, chunk)
			e.moveSurvivorsChunk(workerID, chunk)
			e.workerDoneWg.Done()
		}
	}
}

// initEnvironment configure les sas d'évacuation, la pièce centrale et les obstacles procéduraux.
func (e *Engine) initEnvironment() {
	e.exits = []ExitZone{
		{ID: 1, X: 350.0, Y: 350.0, R: ExitRadius},
		{ID: 2, X: WorldSize - 350.0, Y: 350.0, R: ExitRadius},
		{ID: 3, X: 350.0, Y: WorldSize - 350.0, R: ExitRadius},
		{ID: 4, X: WorldSize - 350.0, Y: WorldSize - 350.0, R: ExitRadius},
	}

	e.centerRoomSize = 900.0
	e.centerRoomX = (WorldSize / 2) - (e.centerRoomSize / 2)
	e.centerRoomY = (WorldSize / 2) - (e.centerRoomSize / 2)
	doorSize := 160.0

	// Points centraux des deux portes pour guider les zombies vers l'extérieur
	e.doorNorthX = e.centerRoomX + (e.centerRoomSize / 2)
	e.doorNorthY = e.centerRoomY - 15.0
	e.doorSouthX = e.centerRoomX + (e.centerRoomSize / 2)
	e.doorSouthY = e.centerRoomY + e.centerRoomSize + 15.0

	e.obstacles = append(e.obstacles,
		// Porte Nord
		Obstacle{X: e.centerRoomX, Y: e.centerRoomY, W: (e.centerRoomSize - doorSize) / 2, H: 30},
		Obstacle{X: e.centerRoomX + (e.centerRoomSize+doorSize)/2, Y: e.centerRoomY, W: (e.centerRoomSize - doorSize) / 2, H: 30},
		// Porte Sud
		Obstacle{X: e.centerRoomX, Y: e.centerRoomY + e.centerRoomSize - 30, W: (e.centerRoomSize - doorSize) / 2, H: 30},
		Obstacle{X: e.centerRoomX + (e.centerRoomSize+doorSize)/2, Y: e.centerRoomY + e.centerRoomSize - 30, W: (e.centerRoomSize - doorSize) / 2, H: 30},
		// Murs latéraux
		Obstacle{X: e.centerRoomX, Y: e.centerRoomY, W: 30, H: e.centerRoomSize},
		Obstacle{X: e.centerRoomX + e.centerRoomSize - 30, Y: e.centerRoomY, W: 30, H: e.centerRoomSize},
	)

	for range ObstacleCount {
		w := 150.0 + e.rng.Float64()*350.0
		h := 80.0 + e.rng.Float64()*300.0
		if e.rng.Float64() > 0.5 {
			w, h = h, w
		}
		ox := 250.0 + e.rng.Float64()*(WorldSize-w-500.0)
		oy := 250.0 + e.rng.Float64()*(WorldSize-h-500.0)

		if ox+w > e.centerRoomX-100 && ox < e.centerRoomX+e.centerRoomSize+100 &&
			oy+h > e.centerRoomY-100 && oy < e.centerRoomY+e.centerRoomSize+100 {
			continue
		}
		e.obstacles = append(e.obstacles, Obstacle{X: ox, Y: oy, W: w, H: h})
	}
}

// initSurvivors instancie l'ensemble des survivants initiaux et leurs attributs.
func (e *Engine) initSurvivors() {
	e.survivors = make([]Survivor, TotalSurvivors)

	for i := range TotalSurvivors {
		var x, y float64
		if i < CenterRoomSurvivors {
			for {
				x = e.centerRoomX + 50.0 + e.rng.Float64()*(e.centerRoomSize-100.0)
				y = e.centerRoomY + 50.0 + e.rng.Float64()*(e.centerRoomSize-100.0)
				if !e.collidesWithObstacleOrClosedExit(x, y) {
					break
				}
			}
		} else {
			for {
				x = 350.0 + e.rng.Float64()*(WorldSize-700.0)
				y = 350.0 + e.rng.Float64()*(WorldSize-700.0)
				if (x < e.centerRoomX || x > e.centerRoomX+e.centerRoomSize || y < e.centerRoomY || y > e.centerRoomY+e.centerRoomSize) &&
					!e.collidesWithObstacleOrClosedExit(x, y) {
					break
				}
			}
		}

		e.survivors[i] = Survivor{
			Alive:         true,
			Escaped:       false,
			ID:            int64(i),
			IsSurvivalist: e.rng.Float64() < SurvivalistRatio,
			IsAlerted:     false,
			IsExhausted:   false,
			Stamina:       MaxStamina - int16(e.rng.Intn(20)),
			X:             x,
			Y:             y,
			Speed:         2.4 + e.rng.Float64()*0.8,
			Fear:          0.9 + e.rng.Float64()*0.4,
			WanderAng:     e.rng.Float64() * 2 * math.Pi,
		}
	}
}

// initHorde initialise les premiers infectés de la simulation.
func (e *Engine) initHorde() {
	e.horde = make([]Zombie, InitialInfected, TotalSurvivors)
	for k := range InitialInfected {
		e.survivors[k].Alive = false
		e.horde[k] = Zombie{
			Active:    true,
			ID:        int64(k),
			X:         e.survivors[k].X,
			Y:         e.survivors[k].Y,
			Speed:     2.3,
			WanderAng: e.rng.Float64() * 2 * math.Pi,
		}
	}
}

func (e *Engine) collidesWithObstacleOrClosedExit(x, y float64) bool {
	if x <= 20 || x >= WorldSize-20 || y <= 20 || y >= WorldSize-20 {
		return true
	}
	for _, obs := range e.obstacles {
		if x >= obs.X && x <= obs.X+obs.W && y >= obs.Y && y <= obs.Y+obs.H {
			return true
		}
	}
	for _, ex := range e.exits {
		if ex.IsClosed {
			dx := ex.X - x
			dy := ex.Y - y
			if dx*dx+dy*dy <= ExitRadiusSq {
				return true
			}
		}
	}
	return false
}

// boundingCells retourne la plage de coordonnées [minCol, maxCol, minRow, maxRow]
// des cellules 2D intersectant le cercle de rayon radius centré en (x, y).
func (e *Engine) boundingCells(x, y, radius float64) (minCol, maxCol, minRow, maxRow int) {
	cellW := WorldSize / float64(GridCols)
	cellH := WorldSize / float64(GridRows)

	minCol = int((x - radius) / cellW)
	maxCol = int((x + radius) / cellW)
	minRow = int((y - radius) / cellH)
	maxRow = int((y + radius) / cellH)

	if minCol < 0 {
		minCol = 0
	}
	if maxCol >= GridCols {
		maxCol = GridCols - 1
	}
	if minRow < 0 {
		minRow = 0
	}
	if maxRow >= GridRows {
		maxRow = GridRows - 1
	}
	return
}

// Update exécute un cycle (tick) de la simulation spatiale en exploitant la grille spatiale 1D
// et en distribuant les lots de survivants sur le Worker Pool borné synchronisé par sync.WaitGroup.
func (e *Engine) Update() {
	e.newZombies = e.newZombies[:0]

	crowdCenterX, crowdCenterY, aliveCount, aliveInRoomCount := e.rebuildSpatialGrid()

	// Distribution des lots de survivants sur le Worker Pool borné
	e.workerDoneWg.Add(e.workerCount)
	for i := 0; i < e.workerCount; i++ {
		e.workerStartCh[i] <- struct{}{}
	}
	e.workerDoneWg.Wait()

	// Réduction sans contention des données produites par les workers
	e.reduceWorkerResults()

	e.updateZombies(aliveCount, aliveInRoomCount, crowdCenterX, crowdCenterY)
}

// reduceWorkerResults agrège les statistiques des workers (rescapés, armement des sas) sans contention.
func (e *Engine) reduceWorkerResults() {
	for w := 0; w < e.workerCount; w++ {
		e.escaped += e.workerEscaped[w]
		e.workerEscaped[w] = 0

		for exIdx := range e.exits {
			if e.workerExitsArmed[w][exIdx] {
				e.exits[exIdx].HasSurvivalistIn = true
				e.workerExitsArmed[w][exIdx] = false
			}
		}
	}
}

// rebuildSpatialGrid réinitialise la grille spatiale, insère les survivants et zombies actifs,
// et calcule le barycentre de la foule ainsi que le nombre de survivants vivants.
func (e *Engine) rebuildSpatialGrid() (crowdCenterX, crowdCenterY float64, aliveCount, aliveInRoomCount int) {
	for i := range e.grid {
		e.grid[i].FirstSurvivor = -1
		e.grid[i].FirstZombie = -1
	}

	var sumX, sumY float64
	cellW := WorldSize / float64(GridCols)
	cellH := WorldSize / float64(GridRows)

	// Insertion des survivants dans la grille + calcul barycentre
	for i := range e.survivors {
		s := &e.survivors[i]
		e.wasAlerted[i] = s.IsAlerted
		e.posSnapshotX[i] = s.X
		e.posSnapshotY[i] = s.Y
		if s.Alive && !s.Escaped {
			sumX += s.X
			sumY += s.Y
			aliveCount++
			if s.X >= e.centerRoomX && s.X <= e.centerRoomX+e.centerRoomSize && s.Y >= e.centerRoomY && s.Y <= e.centerRoomY+e.centerRoomSize {
				aliveInRoomCount++
			}

			col := int(s.X / cellW)
			row := int(s.Y / cellH)
			if col < 0 {
				col = 0
			} else if col >= GridCols {
				col = GridCols - 1
			}
			if row < 0 {
				row = 0
			} else if row >= GridRows {
				row = GridRows - 1
			}
			cellIdx := row*GridCols + col
			e.survivorNext[i] = e.grid[cellIdx].FirstSurvivor
			e.grid[cellIdx].FirstSurvivor = int32(i)
		}
	}

	crowdCenterX = WorldSize / 2
	crowdCenterY = WorldSize / 2
	if aliveCount > 0 {
		crowdCenterX = sumX / float64(aliveCount)
		crowdCenterY = sumY / float64(aliveCount)
	}

	// Insertion de la horde dans la grille
	for i := range e.horde {
		z := &e.horde[i]
		if z.Active {
			col := int(z.X / cellW)
			row := int(z.Y / cellH)
			if col < 0 {
				col = 0
			} else if col >= GridCols {
				col = GridCols - 1
			}
			if row < 0 {
				row = 0
			} else if row >= GridRows {
				row = GridRows - 1
			}
			cellIdx := row*GridCols + col
			e.zombieNext[i] = e.grid[cellIdx].FirstZombie
			e.grid[cellIdx].FirstZombie = int32(i)
		}
	}

	return crowdCenterX, crowdCenterY, aliveCount, aliveInRoomCount
}

// initSurvivorChunks découpe la population totale de survivants en sous-tranches
// indépendantes et équitablement réparties pour les workers (ex. 8 tranches de 1 250 entités).
func (e *Engine) initSurvivorChunks(numWorkers int) {
	if numWorkers <= 0 {
		numWorkers = 1
	}
	e.workerCount = numWorkers
	e.chunks = make([]SurvivorChunk, numWorkers)
	total := len(e.survivors)
	for i := 0; i < numWorkers; i++ {
		start := i * total / numWorkers
		end := (i + 1) * total / numWorkers
		e.chunks[i] = SurvivorChunk{
			WorkerID: i,
			Start:    start,
			End:      end,
		}
	}
}

// SurvivorChunks retourne la liste des sous-tranches indépendantes de survivants.
func (e *Engine) SurvivorChunks() []SurvivorChunk {
	return e.chunks
}

// SurvivorSlice retourne la vue en tranche ([]Survivor) correspondant à un chunk donné.
func (e *Engine) SurvivorSlice(chunk SurvivorChunk) []Survivor {
	return e.survivors[chunk.Start:chunk.End]
}

// updateSurvivorAlertsChunk traite la détection et la panique pour une sous-tranche indépendante de survivants
// en s'appuyant sur le générateur pseudo-aléatoire dédié au worker et les snapshots de position.
func (e *Engine) updateSurvivorAlertsChunk(workerID int, chunk SurvivorChunk) {
	awarenessSq := AwarenessRadiusSq
	panicSpreadSq := PanicSpreadRadiusSq
	rng := e.workerRng[workerID]

	for i := chunk.Start; i < chunk.End; i++ {
		s := &e.survivors[i]
		if !s.Alive || s.Escaped || s.IsAlerted {
			continue
		}

		// Détection de zombies proches
		minCol, maxCol, minRow, maxRow := e.boundingCells(e.posSnapshotX[i], e.posSnapshotY[i], AwarenessRadius)
		for r := minRow; r <= maxRow && !s.IsAlerted; r++ {
			rowOffset := r * GridCols
			for c := minCol; c <= maxCol && !s.IsAlerted; c++ {
				for zIdx := e.grid[rowOffset+c].FirstZombie; zIdx != -1; zIdx = e.zombieNext[zIdx] {
					z := &e.horde[zIdx]
					if !z.Active {
						continue
					}
					dx := e.posSnapshotX[i] - z.X
					dy := e.posSnapshotY[i] - z.Y
					if dx*dx+dy*dy <= awarenessSq {
						s.IsAlerted = true
						break
					}
				}
			}
		}

		// Propagation de la panique par voisins alertés
		if !s.IsAlerted {
			minColP, maxColP, minRowP, maxRowP := e.boundingCells(e.posSnapshotX[i], e.posSnapshotY[i], PanicSpreadRadius)
			for r := minRowP; r <= maxRowP && !s.IsAlerted; r++ {
				rowOffset := r * GridCols
				for c := minColP; c <= maxColP && !s.IsAlerted; c++ {
					for j := e.grid[rowOffset+c].FirstSurvivor; j != -1; j = e.survivorNext[j] {
						if int(j) == i || !e.wasAlerted[j] {
							continue
						}
						other := &e.survivors[j]
						if !other.Alive || other.Escaped {
							continue
						}

						dx := e.posSnapshotX[i] - e.posSnapshotX[j]
						dy := e.posSnapshotY[i] - e.posSnapshotY[j]
						if dx*dx+dy*dy <= panicSpreadSq {
							if rng.Float64() < PanicSpreadChance {
								s.IsAlerted = true
								break
							}
						}
					}
				}
			}
		}
	}
}

// findNearestSurvivalist recherche le survivaliste le plus proche dans un rayon donné.
func (e *Engine) findNearestSurvivalist(sx, sy float64) (leaderIdx int, bestDistSq float64) {
	const searchRadius = 1800.0
	const searchRadiusSq = searchRadius * searchRadius
	bestDistSq = searchRadiusSq
	leaderIdx = -1

	minColL, maxColL, minRowL, maxRowL := e.boundingCells(sx, sy, searchRadius)
	for r := minRowL; r <= maxRowL; r++ {
		rowOffset := r * GridCols
		for c := minColL; c <= maxColL; c++ {
			for j := e.grid[rowOffset+c].FirstSurvivor; j != -1; j = e.survivorNext[j] {
				other := &e.survivors[j]
				if other.Alive && !other.Escaped && other.IsSurvivalist {
					sdx := e.posSnapshotX[j] - sx
					sdy := e.posSnapshotY[j] - sy
					d2 := sdx*sdx + sdy*sdy
					if d2 < bestDistSq {
						bestDistSq = d2
						leaderIdx = int(j)
					}
				}
			}
		}
	}
	return leaderIdx, bestDistSq
}

// moveSurvivorsChunk gère l'évacuation, la fatigue, les trajectoires d'évitement et les déplacements
// pour une sous-tranche indépendante de survivants.
func (e *Engine) moveSurvivorsChunk(workerID int, chunk SurvivorChunk) {
	repelSq := SurvivorRepelRadiusSq
	rng := e.workerRng[workerID]

	for i := chunk.Start; i < chunk.End; i++ {
		s := &e.survivors[i]
		if !s.Alive || s.Escaped {
			continue
		}

		for exIdx := 0; exIdx < len(e.exits); exIdx++ {
			ex := &e.exits[exIdx]
			if ex.IsClosed {
				continue
			}
			dx := ex.X - s.X
			dy := ex.Y - s.Y
			if dx*dx+dy*dy <= ExitRadiusSq {
				s.Escaped = true
				e.workerEscaped[workerID]++
				if s.IsSurvivalist {
					e.workerExitsArmed[workerID][exIdx] = true
				}
				break
			}
		}
		if s.Escaped {
			continue
		}

		// Humain épuisé : arrêt forcé et récupération lente
		if s.IsExhausted {
			s.Stamina += StaminaRecoveryRest
			if s.Stamina >= StaminaResumeSprint {
				s.IsExhausted = false
			}
			continue
		}

		dx, dy := 0.0, 0.0

		if !s.IsAlerted {
			if s.Stamina < MaxStamina {
				s.Stamina++
			}
			s.WanderAng += (rng.Float64() - 0.5) * 0.2
			dx = math.Cos(s.WanderAng) * (s.Speed * 0.4)
			dy = math.Sin(s.WanderAng) * (s.Speed * 0.4)
		} else {
			// Sprint et dépense rapide
			s.Stamina -= StaminaDrainSprint
			if s.Stamina <= 0 {
				s.Stamina = 0
				s.IsExhausted = true
				continue
			}

			fleeX, fleeY := 0.0, 0.0
			threatCount := 0

			minCol, maxCol, minRow, maxRow := e.boundingCells(s.X, s.Y, SurvivorRepelRadius)
			for r := minRow; r <= maxRow; r++ {
				rowOffset := r * GridCols
				for c := minCol; c <= maxCol; c++ {
					for zIdx := e.grid[rowOffset+c].FirstZombie; zIdx != -1; zIdx = e.zombieNext[zIdx] {
						z := &e.horde[zIdx]
						if !z.Active {
							continue
						}
						zdx := s.X - z.X
						zdy := s.Y - z.Y
						zdistSq := zdx*zdx + zdy*zdy
						if zdistSq < repelSq && zdistSq > 0.0001 {
							// Normalisation vectorielle de fuite
							zdist := math.Sqrt(zdistSq)
							fleeX += (zdx / zdist)
							fleeY += (zdy / zdist)
							threatCount++
						}
					}
				}
			}

			if threatCount > 0 {
				flen := math.Sqrt(fleeX*fleeX + fleeY*fleeY)
				dx = (fleeX / flen) * s.Speed * 1.3
				dy = (fleeY / flen) * s.Speed * 1.3
			} else if s.IsSurvivalist {
				bestDistSq := math.MaxFloat64
				var targetExit *ExitZone
				for exIdx := 0; exIdx < len(e.exits); exIdx++ {
					ex := &e.exits[exIdx]
					if ex.IsClosed {
						continue
					}
					edx := ex.X - s.X
					edy := ex.Y - s.Y
					d2 := edx*edx + edy*edy
					if d2 < bestDistSq {
						bestDistSq = d2
						targetExit = ex
					}
				}

				if targetExit != nil {
					bestDist := math.Sqrt(bestDistSq)
					dx = ((targetExit.X - s.X) / bestDist) * s.Speed * s.Fear
					dy = ((targetExit.Y - s.Y) / bestDist) * s.Speed * s.Fear
				}
			} else {
				leaderIdx, bestSurvDistSq := e.findNearestSurvivalist(s.X, s.Y)
				if leaderIdx >= 0 {
					leaderX := e.posSnapshotX[leaderIdx]
					leaderY := e.posSnapshotY[leaderIdx]
					bestDist := math.Sqrt(bestSurvDistSq)
					dx = ((leaderX - s.X) / bestDist) * s.Speed * s.Fear
					dy = ((leaderY - s.Y) / bestDist) * s.Speed * s.Fear
				} else {
					s.WanderAng += (rng.Float64() - 0.5) * 0.5
					dx = math.Cos(s.WanderAng) * s.Speed * 0.8
					dy = math.Sin(s.WanderAng) * s.Speed * 0.8
				}
			}
		}

		dx += (rng.Float64() - 0.5) * 0.8
		dy += (rng.Float64() - 0.5) * 0.8

		nextX := s.X + dx
		nextY := s.Y + dy

		if !e.collidesWithObstacleOrClosedExit(nextX, s.Y) {
			s.X = nextX
		}
		if !e.collidesWithObstacleOrClosedExit(s.X, nextY) {
			s.Y = nextY
		}
	}
}

// findClosestSurvivor localise le survivant vivant le plus proche à portée de vue du zombie.
func (e *Engine) findClosestSurvivor(zx, zy, visionSq float64, inRoom bool, aliveInRoomCount int) (closestIdx int, closestDistSq float64) {
	closestDistSq = visionSq
	closestIdx = -1

	minCol, maxCol, minRow, maxRow := e.boundingCells(zx, zy, ZombieVisionRadius)
	for r := minRow; r <= maxRow; r++ {
		rowOffset := r * GridCols
		for c := minCol; c <= maxCol; c++ {
			for j := e.grid[rowOffset+c].FirstSurvivor; j != -1; j = e.survivorNext[j] {
				s := &e.survivors[j]
				if !s.Alive || s.Escaped {
					continue
				}

				// Si le zombie est enfermé et qu'il n'y a plus d'humains vivants dans la pièce, il ignore ceux de dehors pour d'abord sortir
				if inRoom && aliveInRoomCount == 0 {
					continue
				}

				dx := s.X - zx
				dy := s.Y - zy
				d2 := dx*dx + dy*dy

				if d2 < closestDistSq {
					closestDistSq = d2
					closestIdx = int(j)
				}
			}
		}
	}
	return closestIdx, closestDistSq
}

// updateZombies pilote la traque, les infections et les mouvements de la horde.
func (e *Engine) updateZombies(aliveCount, aliveInRoomCount int, crowdCenterX, crowdCenterY float64) {
	visionSq := ZombieVisionRadiusSq
	sealSq := ExitSealZombieRadiusSq

	for i := 0; i < len(e.horde); i++ {
		z := &e.horde[i]
		if !z.Active {
			continue
		}

		for exIdx := 0; exIdx < len(e.exits); exIdx++ {
			ex := &e.exits[exIdx]
			if ex.HasSurvivalistIn && !ex.IsClosed {
				edx := ex.X - z.X
				edy := ex.Y - z.Y
				if edx*edx+edy*edy <= sealSq {
					ex.IsClosed = true
				}
			}
		}

		inRoom := (z.X >= e.centerRoomX && z.X <= e.centerRoomX+e.centerRoomSize && z.Y >= e.centerRoomY && z.Y <= e.centerRoomY+e.centerRoomSize)

		closestIdx, closestDistSq := e.findClosestSurvivor(z.X, z.Y, visionSq, inRoom, aliveInRoomCount)

		dx, dy := 0.0, 0.0

		// Cas 1 : Cible humaine directe et visible
		if closestIdx >= 0 {
			target := &e.survivors[closestIdx]

			// Test direct au carré pour l'infection
			if closestDistSq <= InfectionRadiusSq {
				target.Alive = false
				e.newZombies = append(e.newZombies, Zombie{
					Active:    true,
					ID:        int64(len(e.horde) + len(e.newZombies)),
					X:         target.X,
					Y:         target.Y,
					Speed:     1.9 + e.rng.Float64()*0.7,
					WanderAng: e.rng.Float64() * 2 * math.Pi,
				})
			}

			// Normalisation vectorielle nécessaire pour orienter la vitesse du zombie
			closestDist := math.Sqrt(closestDistSq)
			dx = ((target.X - z.X) / closestDist) * z.Speed
			dy = ((target.Y - z.Y) / closestDist) * z.Speed
		} else if inRoom && aliveInRoomCount == 0 {
			// Cas 2 : Zombie enfermé dans une pièce dépeuplée -> cap direct sur la porte la plus proche
			targetDoorX := e.doorNorthX
			targetDoorY := e.doorNorthY
			if math.Abs(z.Y-e.doorSouthY) < math.Abs(z.Y-e.doorNorthY) {
				targetDoorX = e.doorSouthX
				targetDoorY = e.doorSouthY
			}

			ddx := targetDoorX - z.X
			ddy := targetDoorY - z.Y
			ddistSq := ddx*ddx + ddy*ddy
			// Test direct de distance au carré (> 2.0^2 = 4.0), racine calculée uniquement pour normaliser
			if ddistSq > 4.0 {
				ddist := math.Sqrt(ddistSq)
				dx = (ddx / ddist) * z.Speed
				dy = (ddy / ddist) * z.Speed
			}
		} else if aliveCount > 0 {
			// Cas 3 : À l'extérieur ou foule en approche -> cap vers le centre de gravité humain
			cdx := crowdCenterX - z.X
			cdy := crowdCenterY - z.Y
			cdistSq := cdx*cdx + cdy*cdy
			// Test direct au carré (> 1.0^2 = 1.0), Atan2 se charge de la direction sans racine carrée
			if cdistSq > 1.0 {
				angleToCrowd := math.Atan2(cdy, cdx) + (e.rng.Float64()-0.5)*0.6
				dx = math.Cos(angleToCrowd) * z.Speed * 0.9
				dy = math.Sin(angleToCrowd) * z.Speed * 0.9
			}
		} else {
			z.WanderAng += (e.rng.Float64() - 0.5) * 0.4
			dx = math.Cos(z.WanderAng) * (z.Speed * 0.6)
			dy = math.Sin(z.WanderAng) * (z.Speed * 0.6)
		}

		nextX := z.X + dx
		nextY := z.Y + dy

		if !e.collidesWithObstacleOrClosedExit(nextX, z.Y) {
			z.X = nextX
		} else {
			z.WanderAng += math.Pi * 0.5
		}

		if !e.collidesWithObstacleOrClosedExit(z.X, nextY) {
			z.Y = nextY
		} else {
			z.WanderAng += math.Pi * 0.5
		}
	}

	if len(e.newZombies) > 0 {
		e.horde = append(e.horde, e.newZombies...)
	}
}

// GenerateHeatmap génère la grille de densité de la horde de zombies.
func (e *Engine) GenerateHeatmap() []int {
	clear(e.heatmap[:])
	cellW := WorldSize / float64(GridCols)
	cellH := WorldSize / float64(GridRows)

	for _, z := range e.horde {
		if !z.Active {
			continue
		}
		gx := int(z.X / cellW)
		gy := int(z.Y / cellH)

		if gx >= 0 && gx < GridCols && gy >= 0 && gy < GridRows {
			e.heatmap[gy*GridCols+gx]++
		}
	}
	return e.heatmap[:]
}

// GetSurvivorCounts calcule et retourne les métriques de répartition de la population humaine.
func (e *Engine) GetSurvivorCounts() (healthy, survs, normals, alerted, exhausted int) {
	for _, s := range e.survivors {
		if s.Alive && !s.Escaped {
			healthy++
			if s.IsSurvivalist {
				survs++
			} else {
				normals++
			}
			if s.IsAlerted {
				alerted++
			}
			if s.IsExhausted {
				exhausted++
			}
		}
	}
	return
}

// GetOpenExitsCount compte le nombre de sas d'évacuation encore ouverts.
func (e *Engine) GetOpenExitsCount() int {
	c := 0
	for _, ex := range e.exits {
		if !ex.IsClosed {
			c++
		}
	}
	return c
}

// BuildFramePayload produit le payload JSON complet d'un tick pour la télémétrie.
func (e *Engine) BuildFramePayload(currentTPS int) FramePayload {
	healthy, survCount, normCount, alertedCount, exhaustedCount := e.GetSurvivorCounts()
	isFinished := (healthy == 0)

	e.survPoints = e.survPoints[:0]
	for _, s := range e.survivors {
		if s.Alive && !s.Escaped {
			e.survPoints = append(e.survPoints, Point{
				X:             int16(s.X),
				Y:             int16(s.Y),
				IsSurvivalist: s.IsSurvivalist,
				IsAlerted:     s.IsAlerted,
				IsExhausted:   s.IsExhausted,
			})
		}
	}

	processed := e.escaped + (len(e.horde) - InitialInfected)
	ratio := 0.0
	if processed > 0 {
		ratio = (float64(e.escaped) / float64(processed)) * 100.0
	}

	return FramePayload{
		TPS:       currentTPS,
		WorldSize: WorldSize,
		GridCols:  GridCols,
		GridRows:  GridRows,
		Stats: Stats{
			Seed:          MasterSeed,
			Infected:      len(e.horde),
			Healthy:       healthy,
			Alerted:       alertedCount,
			Exhausted:     exhaustedCount,
			Survivalists:  survCount,
			Normals:       normCount,
			Escaped:       e.escaped,
			OpenExits:     e.GetOpenExitsCount(),
			SurvivalRatio: math.Round(ratio*10) / 10,
			IsFinished:    isFinished,
		},
		Obstacles: e.obstacles,
		Exits:     e.exits,
		Heatmap:   e.GenerateHeatmap(),
		Survivors: e.survPoints,
	}
}

// Instance globale par défaut pour la compatibilité avec l'API package-level.
var defaultEngine *Engine

func InitSimulation(seed int64) {
	defaultEngine = NewEngine(seed)
}

func UpdateSimulation() {
	if defaultEngine != nil {
		defaultEngine.Update()
	}
}

func DefaultEngine() *Engine {
	return defaultEngine
}
