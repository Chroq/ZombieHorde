package simulation

import (
	"math"
	"math/rand"
)

// Engine encapsule l'état complet et le moteur de calcul d'une simulation ZombieHorde.
type Engine struct {
	rng            *rand.Rand
	obstacles      []Obstacle
	horde          []Zombie
	exits          []ExitZone
	newZombies     []Zombie
	survivors      []Survivor
	heatmap        [GridCols * GridRows]int
	survPoints     []Point
	escaped        int
	centerRoomX    float64
	centerRoomY    float64
	centerRoomSize float64
	doorNorthX     float64
	doorNorthY     float64
	doorSouthX     float64
	doorSouthY     float64
	grid           [GridCols * GridRows]Cell
	survivorNext   [TotalSurvivors]int32
	zombieNext     [TotalSurvivors]int32
	wasAlerted     [TotalSurvivors]bool
}

// NewEngine instancie et initialise une nouvelle simulation déterministe à partir d'une graine.
func NewEngine(seed int64) *Engine {
	e := &Engine{
		rng:        rand.New(rand.NewSource(seed)),
		escaped:    0,
		obstacles:  make([]Obstacle, 0, 6+ObstacleCount),
		newZombies: make([]Zombie, 0, TotalSurvivors),
		survPoints: make([]Point, 0, TotalSurvivors),
	}

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

	return e
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
			if math.Sqrt(dx*dx+dy*dy) <= ex.R {
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

// Update exécute un cycle (tick) de la simulation spatiale en exploitant la grille spatiale 1D.
func (e *Engine) Update() {
	e.newZombies = e.newZombies[:0]

	// 0. Réinitialisation de la grille spatiale 1D
	for i := range e.grid {
		e.grid[i].FirstSurvivor = -1
		e.grid[i].FirstZombie = -1
	}

	var sumX, sumY float64
	var aliveCount float64
	var aliveInRoomCount float64

	cellW := WorldSize / float64(GridCols)
	cellH := WorldSize / float64(GridRows)

	// Insertion des survivants dans la grille + calcul barycentre
	for i := range e.survivors {
		s := &e.survivors[i]
		e.wasAlerted[i] = s.IsAlerted
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

	crowdCenterX := WorldSize / 2
	crowdCenterY := WorldSize / 2
	if aliveCount > 0 {
		crowdCenterX = sumX / aliveCount
		crowdCenterY = sumY / aliveCount
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

	// 1. Détection & Alerte irréversible via la grille spatiale
	awarenessSq := AwarenessRadius * AwarenessRadius
	panicSpreadSq := PanicSpreadRadius * PanicSpreadRadius

	for i := range e.survivors {
		s := &e.survivors[i]
		if !s.Alive || s.Escaped || s.IsAlerted {
			continue
		}

		// Détection de zombies proches
		minCol, maxCol, minRow, maxRow := e.boundingCells(s.X, s.Y, AwarenessRadius)
		for r := minRow; r <= maxRow && !s.IsAlerted; r++ {
			rowOffset := r * GridCols
			for c := minCol; c <= maxCol && !s.IsAlerted; c++ {
				for zIdx := e.grid[rowOffset+c].FirstZombie; zIdx != -1; zIdx = e.zombieNext[zIdx] {
					z := &e.horde[zIdx]
					if !z.Active {
						continue
					}
					dx := s.X - z.X
					dy := s.Y - z.Y
					if dx*dx+dy*dy <= awarenessSq {
						s.IsAlerted = true
						break
					}
				}
			}
		}

		// Propagation de la panique par voisins alertés
		if !s.IsAlerted {
			minColP, maxColP, minRowP, maxRowP := e.boundingCells(s.X, s.Y, PanicSpreadRadius)
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

						dx := s.X - other.X
						dy := s.Y - other.Y
						if dx*dx+dy*dy <= panicSpreadSq {
							if e.rng.Float64() < PanicSpreadChance {
								s.IsAlerted = true
								break
							}
						}
					}
				}
			}
		}
	}

	// 2. Déplacement des survivants avec pénalité de fatigue
	repelSq := SurvivorRepelRadius * SurvivorRepelRadius

	for i := range e.survivors {
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
			if dx*dx+dy*dy <= ex.R*ex.R {
				s.Escaped = true
				e.escaped++
				if s.IsSurvivalist {
					ex.HasSurvivalistIn = true
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
			s.WanderAng += (e.rng.Float64() - 0.5) * 0.2
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
				bestSurvDistSq := 1800.0 * 1800.0
				leaderIdx := -1

				minColL, maxColL, minRowL, maxRowL := e.boundingCells(s.X, s.Y, 1800.0)
				for r := minRowL; r <= maxRowL; r++ {
					rowOffset := r * GridCols
					for c := minColL; c <= maxColL; c++ {
						for j := e.grid[rowOffset+c].FirstSurvivor; j != -1; j = e.survivorNext[j] {
							other := &e.survivors[j]
							if other.Alive && !other.Escaped && other.IsSurvivalist {
								sdx := other.X - s.X
								sdy := other.Y - s.Y
								d2 := sdx*sdx + sdy*sdy
								if d2 < bestSurvDistSq {
									bestSurvDistSq = d2
									leaderIdx = int(j)
								}
							}
						}
					}
				}

				if leaderIdx >= 0 {
					leader := &e.survivors[leaderIdx]
					bestDist := math.Sqrt(bestSurvDistSq)
					dx = ((leader.X - s.X) / bestDist) * s.Speed * s.Fear
					dy = ((leader.Y - s.Y) / bestDist) * s.Speed * s.Fear
				} else {
					s.WanderAng += (e.rng.Float64() - 0.5) * 0.5
					dx = math.Cos(s.WanderAng) * s.Speed * 0.8
					dy = math.Sin(s.WanderAng) * s.Speed * 0.8
				}
			}
		}

		dx += (e.rng.Float64() - 0.5) * 0.8
		dy += (e.rng.Float64() - 0.5) * 0.8

		nextX := s.X + dx
		nextY := s.Y + dy

		if !e.collidesWithObstacleOrClosedExit(nextX, s.Y) {
			s.X = nextX
		}
		if !e.collidesWithObstacleOrClosedExit(s.X, nextY) {
			s.Y = nextY
		}
	}

	// 3. Traque des zombies via la grille spatiale
	visionSq := ZombieVisionRadius * ZombieVisionRadius
	sealSq := ExitSealZombieRadius * ExitSealZombieRadius

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

		closestDistSq := visionSq
		closestIdx := -1

		minCol, maxCol, minRow, maxRow := e.boundingCells(z.X, z.Y, ZombieVisionRadius)
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

					dx := s.X - z.X
					dy := s.Y - z.Y
					d2 := dx*dx + dy*dy

					if d2 < closestDistSq {
						closestDistSq = d2
						closestIdx = int(j)
					}
				}
			}
		}

		dx, dy := 0.0, 0.0

		// Cas 1 : Cible humaine directe et visible
		if closestIdx >= 0 {
			closestDist := math.Sqrt(closestDistSq)
			target := &e.survivors[closestIdx]
			dx = ((target.X - z.X) / closestDist) * z.Speed
			dy = ((target.Y - z.Y) / closestDist) * z.Speed

			if closestDist <= InfectionRadius {
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
			ddist := math.Sqrt(ddx*ddx + ddy*ddy)
			if ddist > 2.0 {
				dx = (ddx / ddist) * z.Speed
				dy = (ddy / ddist) * z.Speed
			}
		} else if aliveCount > 0 {
			// Cas 3 : À l'extérieur ou foule en approche -> cap vers le centre de gravité humain
			cdx := crowdCenterX - z.X
			cdy := crowdCenterY - z.Y
			cdist := math.Sqrt(cdx*cdx + cdy*cdy)
			if cdist > 1.0 {
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
