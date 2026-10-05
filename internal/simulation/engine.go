package simulation

import (
	"math"
	"math/rand"
)

// Engine encapsule l'état complet et le moteur de calcul d'une simulation ZombieHorde.
type Engine struct {
	rng            *rand.Rand
	horde          []Zombie
	survivors      []Survivor
	obstacles      []Obstacle
	exits          []ExitZone
	escaped        int
	centerRoomX    float64
	centerRoomY    float64
	centerRoomSize float64
	doorNorthX     float64
	doorNorthY     float64
	doorSouthX     float64
	doorSouthY     float64
}

// NewEngine instancie et initialise une nouvelle simulation déterministe à partir d'une graine.
func NewEngine(seed int64) *Engine {
	e := &Engine{
		rng:     rand.New(rand.NewSource(seed)),
		escaped: 0,
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

	e.obstacles = []Obstacle{
		// Porte Nord
		{X: e.centerRoomX, Y: e.centerRoomY, W: (e.centerRoomSize - doorSize) / 2, H: 30},
		{X: e.centerRoomX + (e.centerRoomSize+doorSize)/2, Y: e.centerRoomY, W: (e.centerRoomSize - doorSize) / 2, H: 30},
		// Porte Sud
		{X: e.centerRoomX, Y: e.centerRoomY + e.centerRoomSize - 30, W: (e.centerRoomSize - doorSize) / 2, H: 30},
		{X: e.centerRoomX + (e.centerRoomSize+doorSize)/2, Y: e.centerRoomY + e.centerRoomSize - 30, W: (e.centerRoomSize - doorSize) / 2, H: 30},
		// Murs latéraux
		{X: e.centerRoomX, Y: e.centerRoomY, W: 30, H: e.centerRoomSize},
		{X: e.centerRoomX + e.centerRoomSize - 30, Y: e.centerRoomY, W: 30, H: e.centerRoomSize},
	}

	for i := 0; i < ObstacleCount; i++ {
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

	for i := 0; i < TotalSurvivors; i++ {
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

	e.horde = make([]Zombie, InitialInfected)
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

// Update exécute un cycle (tick) de la simulation spatiale.
func (e *Engine) Update() {
	var sumX, sumY float64
	var aliveCount float64
	var aliveInRoomCount float64

	for i := 0; i < len(e.survivors); i++ {
		if e.survivors[i].Alive && !e.survivors[i].Escaped {
			sumX += e.survivors[i].X
			sumY += e.survivors[i].Y
			aliveCount++
			if e.survivors[i].X >= e.centerRoomX && e.survivors[i].X <= e.centerRoomX+e.centerRoomSize && e.survivors[i].Y >= e.centerRoomY && e.survivors[i].Y <= e.centerRoomY+e.centerRoomSize {
				aliveInRoomCount++
			}
		}
	}
	crowdCenterX := WorldSize / 2
	crowdCenterY := WorldSize / 2
	if aliveCount > 0 {
		crowdCenterX = sumX / aliveCount
		crowdCenterY = sumY / aliveCount
	}

	wasAlerted := make([]bool, len(e.survivors))
	for i := range e.survivors {
		wasAlerted[i] = e.survivors[i].IsAlerted
	}

	// 1. Détection & Alerte irréversible
	for i := 0; i < len(e.survivors); i++ {
		if !e.survivors[i].Alive || e.survivors[i].Escaped || e.survivors[i].IsAlerted {
			continue
		}

		for zIdx := 0; zIdx < len(e.horde); zIdx++ {
			z := &e.horde[zIdx]
			if !z.Active {
				continue
			}
			dx := e.survivors[i].X - z.X
			dy := e.survivors[i].Y - z.Y
			if dx*dx+dy*dy <= AwarenessRadius*AwarenessRadius {
				e.survivors[i].IsAlerted = true
				break
			}
		}

		if !e.survivors[i].IsAlerted {
			for j := 0; j < len(e.survivors); j++ {
				if i == j || !wasAlerted[j] {
					continue
				}
				if !e.survivors[j].Alive || e.survivors[j].Escaped {
					continue
				}

				dx := e.survivors[i].X - e.survivors[j].X
				dy := e.survivors[i].Y - e.survivors[j].Y
				if dx*dx+dy*dy <= PanicSpreadRadius*PanicSpreadRadius {
					if e.rng.Float64() < PanicSpreadChance {
						e.survivors[i].IsAlerted = true
						break
					}
				}
			}
		}
	}

	// 2. Déplacement des survivants avec pénalité de fatigue lourde
	for i := 0; i < len(e.survivors); i++ {
		if !e.survivors[i].Alive || e.survivors[i].Escaped {
			continue
		}

		for exIdx := 0; exIdx < len(e.exits); exIdx++ {
			ex := &e.exits[exIdx]
			if ex.IsClosed {
				continue
			}
			dx := ex.X - e.survivors[i].X
			dy := ex.Y - e.survivors[i].Y
			if math.Sqrt(dx*dx+dy*dy) <= ex.R {
				e.survivors[i].Escaped = true
				e.escaped++
				if e.survivors[i].IsSurvivalist {
					ex.HasSurvivalistIn = true
				}
				break
			}
		}
		if e.survivors[i].Escaped {
			continue
		}

		// Humain épuisé : arrêt forcé et récupération lente
		if e.survivors[i].IsExhausted {
			e.survivors[i].Stamina += StaminaRecoveryRest
			if e.survivors[i].Stamina >= StaminaResumeSprint {
				e.survivors[i].IsExhausted = false
			}
			continue
		}

		dx, dy := 0.0, 0.0

		if !e.survivors[i].IsAlerted {
			if e.survivors[i].Stamina < MaxStamina {
				e.survivors[i].Stamina++
			}
			e.survivors[i].WanderAng += (e.rng.Float64() - 0.5) * 0.2
			dx = math.Cos(e.survivors[i].WanderAng) * (e.survivors[i].Speed * 0.4)
			dy = math.Sin(e.survivors[i].WanderAng) * (e.survivors[i].Speed * 0.4)
		} else {
			// Sprint et dépense rapide
			e.survivors[i].Stamina -= StaminaDrainSprint
			if e.survivors[i].Stamina <= 0 {
				e.survivors[i].Stamina = 0
				e.survivors[i].IsExhausted = true
				continue
			}

			fleeX, fleeY := 0.0, 0.0
			threatCount := 0
			for zIdx := 0; zIdx < len(e.horde); zIdx++ {
				z := &e.horde[zIdx]
				if !z.Active {
					continue
				}
				zdx := e.survivors[i].X - z.X
				zdy := e.survivors[i].Y - z.Y
				zdist := math.Sqrt(zdx*zdx + zdy*zdy)
				if zdist < SurvivorRepelRadius && zdist > 0.001 {
					fleeX += (zdx / zdist)
					fleeY += (zdy / zdist)
					threatCount++
				}
			}

			if threatCount > 0 {
				flen := math.Sqrt(fleeX*fleeX + fleeY*fleeY)
				dx = (fleeX / flen) * e.survivors[i].Speed * 1.3
				dy = (fleeY / flen) * e.survivors[i].Speed * 1.3
			} else if e.survivors[i].IsSurvivalist {
				bestDist := math.MaxFloat64
				var targetExit *ExitZone
				for exIdx := 0; exIdx < len(e.exits); exIdx++ {
					ex := &e.exits[exIdx]
					if ex.IsClosed {
						continue
					}
					edx := ex.X - e.survivors[i].X
					edy := ex.Y - e.survivors[i].Y
					d := math.Sqrt(edx*edx + edy*edy)
					if d < bestDist {
						bestDist = d
						targetExit = ex
					}
				}

				if targetExit != nil {
					dx = ((targetExit.X - e.survivors[i].X) / bestDist) * e.survivors[i].Speed * e.survivors[i].Fear
					dy = ((targetExit.Y - e.survivors[i].Y) / bestDist) * e.survivors[i].Speed * e.survivors[i].Fear
				}
			} else {
				bestSurvDist := math.MaxFloat64
				leaderIdx := -1

				for j := 0; j < len(e.survivors); j++ {
					other := &e.survivors[j]
					if other.Alive && !other.Escaped && other.IsSurvivalist {
						sdx := other.X - e.survivors[i].X
						sdy := other.Y - e.survivors[i].Y
						d := math.Sqrt(sdx*sdx + sdy*sdy)
						if d < bestSurvDist {
							bestSurvDist = d
							leaderIdx = j
						}
					}
				}

				if leaderIdx >= 0 && bestSurvDist < 1800.0 {
					leader := &e.survivors[leaderIdx]
					dx = ((leader.X - e.survivors[i].X) / bestSurvDist) * e.survivors[i].Speed * e.survivors[i].Fear
					dy = ((leader.Y - e.survivors[i].Y) / bestSurvDist) * e.survivors[i].Speed * e.survivors[i].Fear
				} else {
					e.survivors[i].WanderAng += (e.rng.Float64() - 0.5) * 0.5
					dx = math.Cos(e.survivors[i].WanderAng) * e.survivors[i].Speed * 0.8
					dy = math.Sin(e.survivors[i].WanderAng) * e.survivors[i].Speed * 0.8
				}
			}
		}

		dx += (e.rng.Float64() - 0.5) * 0.8
		dy += (e.rng.Float64() - 0.5) * 0.8

		nextX := e.survivors[i].X + dx
		nextY := e.survivors[i].Y + dy

		if !e.collidesWithObstacleOrClosedExit(nextX, e.survivors[i].Y) {
			e.survivors[i].X = nextX
		}
		if !e.collidesWithObstacleOrClosedExit(e.survivors[i].X, nextY) {
			e.survivors[i].Y = nextY
		}
	}

	// 3. Traque des zombies : priorité à la sortie de la pièce si vide
	var newZombies []Zombie

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
				if math.Sqrt(edx*edx+edy*edy) <= ExitSealZombieRadius {
					ex.IsClosed = true
				}
			}
		}

		inRoom := (z.X >= e.centerRoomX && z.X <= e.centerRoomX+e.centerRoomSize && z.Y >= e.centerRoomY && z.Y <= e.centerRoomY+e.centerRoomSize)

		closestDist := math.MaxFloat64
		closestIdx := -1

		for j := 0; j < len(e.survivors); j++ {
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
			dist := math.Sqrt(dx*dx + dy*dy)

			if dist < closestDist {
				closestDist = dist
				closestIdx = j
			}
		}

		dx, dy := 0.0, 0.0

		// Cas 1 : Cible humaine directe et visible
		if closestIdx >= 0 && closestDist < ZombieVisionRadius {
			target := &e.survivors[closestIdx]
			dx = ((target.X - z.X) / closestDist) * z.Speed
			dy = ((target.Y - z.Y) / closestDist) * z.Speed

			if closestDist <= InfectionRadius {
				target.Alive = false
				newZombies = append(newZombies, Zombie{
					Active:    true,
					ID:        int64(len(e.horde) + len(newZombies)),
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

	if len(newZombies) > 0 {
		e.horde = append(e.horde, newZombies...)
	}
}

// GenerateHeatmap génère la grille de densité de la horde de zombies.
func (e *Engine) GenerateHeatmap() []int {
	grid := make([]int, GridCols*GridRows)
	cellW := WorldSize / float64(GridCols)
	cellH := WorldSize / float64(GridRows)

	for _, z := range e.horde {
		if !z.Active {
			continue
		}
		gx := int(z.X / cellW)
		gy := int(z.Y / cellH)

		if gx >= 0 && gx < GridCols && gy >= 0 && gy < GridRows {
			grid[gy*GridCols+gx]++
		}
	}
	return grid
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

	survPoints := make([]Point, 0, healthy)
	for _, s := range e.survivors {
		if s.Alive && !s.Escaped {
			survPoints = append(survPoints, Point{
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
		Survivors: survPoints,
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
