package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"

	"golang.org/x/net/websocket"
)

const (
	MasterSeed           int64   = 42
	WorldSize            float64 = 5000.0
	GridCols             int     = 100
	GridRows             int     = 100
	TotalSurvivors       int     = 10000
	CenterRoomSurvivors  int     = 1500
	InitialInfected      int     = 5
	SurvivalistRatio     float64 = 0.12
	ObstacleCount        int     = 35
	ExitRadius           float64 = 75.0
	TargetFPS            int     = 30
	InfectionRadius      float64 = 22.0
	ZombieVisionRadius   float64 = 1600.0
	SurvivorRepelRadius  float64 = 350.0
	ExitSealZombieRadius float64 = 250.0
	AwarenessRadius      float64 = 160.0
	PanicSpreadRadius    float64 = 70.0
	PanicSpreadChance    float64 = 0.05

	// Pénalité de fatigue sévère
	MaxStamina          int16 = 100
	StaminaDrainSprint  int16 = 2  // Épuisement deux fois plus rapide en panique
	StaminaRecoveryRest int16 = 1  // Récupération lente au repos
	StaminaResumeSprint int16 = 60 // Immobilisation prolongée (~2 secondes sans bouger)
)

type Obstacle struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type ExitZone struct {
	ID               float64 `json:"id"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	R                float64 `json:"r"`
	HasSurvivalistIn bool    `json:"has_survivalist_in"`
	IsClosed         bool    `json:"is_closed"`
}

type Zombie struct {
	Active    bool
	ID        int64
	X, Y      float64
	Speed     float64
	WanderAng float64
}

type Survivor struct {
	Escaped       bool
	ID            int64
	Alive         bool
	IsSurvivalist bool
	IsAlerted     bool
	IsExhausted   bool
	Stamina       int16
	X, Y          float64
	Speed         float64
	Fear          float64
	WanderAng     float64
}

type Point struct {
	X             int16 `json:"x"`
	Y             int16 `json:"y"`
	IsSurvivalist bool  `json:"is_survivalist"`
	IsAlerted     bool  `json:"is_alerted"`
	IsExhausted   bool  `json:"is_exhausted"`
}

type Stats struct {
	Seed          int64   `json:"seed"`
	Infected      int     `json:"infected"`
	Healthy       int     `json:"healthy"`
	Alerted       int     `json:"alerted"`
	Exhausted     int     `json:"exhausted"`
	Survivalists  int     `json:"survivalists"`
	Normals       int     `json:"normals"`
	Escaped       int     `json:"escaped"`
	OpenExits     int     `json:"open_exits"`
	SurvivalRatio float64 `json:"survival_ratio"`
	IsFinished    bool    `json:"is_finished"`
}

type FramePayload struct {
	TPS       int        `json:"tps"`
	WorldSize float64    `json:"world_size"`
	GridCols  int        `json:"grid_cols"`
	GridRows  int        `json:"grid_rows"`
	Stats     Stats      `json:"stats"`
	Obstacles []Obstacle `json:"obstacles"`
	Exits     []ExitZone `json:"exits"`
	Heatmap   []int      `json:"heatmap"`
	Survivors []Point    `json:"survivors"`
}

var (
	simRng         *rand.Rand
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
)

func collidesWithObstacleOrClosedExit(x, y float64) bool {
	if x <= 20 || x >= WorldSize-20 || y <= 20 || y >= WorldSize-20 {
		return true
	}
	for _, obs := range obstacles {
		if x >= obs.X && x <= obs.X+obs.W && y >= obs.Y && y <= obs.Y+obs.H {
			return true
		}
	}
	for _, ex := range exits {
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

func InitSimulation(seed int64) {
	simRng = rand.New(rand.NewSource(seed))
	escaped = 0

	exits = []ExitZone{
		{ID: 1, X: 350.0, Y: 350.0, R: ExitRadius},
		{ID: 2, X: WorldSize - 350.0, Y: 350.0, R: ExitRadius},
		{ID: 3, X: 350.0, Y: WorldSize - 350.0, R: ExitRadius},
		{ID: 4, X: WorldSize - 350.0, Y: WorldSize - 350.0, R: ExitRadius},
	}

	centerRoomSize = 900.0
	centerRoomX = (WorldSize / 2) - (centerRoomSize / 2)
	centerRoomY = (WorldSize / 2) - (centerRoomSize / 2)
	doorSize := 160.0

	// Points centraux des deux portes pour guider les zombies vers l'extérieur
	doorNorthX = centerRoomX + (centerRoomSize / 2)
	doorNorthY = centerRoomY - 15.0
	doorSouthX = centerRoomX + (centerRoomSize / 2)
	doorSouthY = centerRoomY + centerRoomSize + 15.0

	obstacles = []Obstacle{
		// Porte Nord
		{X: centerRoomX, Y: centerRoomY, W: (centerRoomSize - doorSize) / 2, H: 30},
		{X: centerRoomX + (centerRoomSize+doorSize)/2, Y: centerRoomY, W: (centerRoomSize - doorSize) / 2, H: 30},
		// Porte Sud
		{X: centerRoomX, Y: centerRoomY + centerRoomSize - 30, W: (centerRoomSize - doorSize) / 2, H: 30},
		{X: centerRoomX + (centerRoomSize+doorSize)/2, Y: centerRoomY + centerRoomSize - 30, W: (centerRoomSize - doorSize) / 2, H: 30},
		// Murs latéraux
		{X: centerRoomX, Y: centerRoomY, W: 30, H: centerRoomSize},
		{X: centerRoomX + centerRoomSize - 30, Y: centerRoomY, W: 30, H: centerRoomSize},
	}

	for i := 0; i < ObstacleCount; i++ {
		w := 150.0 + simRng.Float64()*350.0
		h := 80.0 + simRng.Float64()*300.0
		if simRng.Float64() > 0.5 {
			w, h = h, w
		}
		ox := 250.0 + simRng.Float64()*(WorldSize-w-500.0)
		oy := 250.0 + simRng.Float64()*(WorldSize-h-500.0)

		if ox+w > centerRoomX-100 && ox < centerRoomX+centerRoomSize+100 &&
			oy+h > centerRoomY-100 && oy < centerRoomY+centerRoomSize+100 {
			continue
		}
		obstacles = append(obstacles, Obstacle{X: ox, Y: oy, W: w, H: h})
	}

	survivors = make([]Survivor, TotalSurvivors)

	for i := 0; i < TotalSurvivors; i++ {
		var x, y float64
		if i < CenterRoomSurvivors {
			for {
				x = centerRoomX + 50.0 + simRng.Float64()*(centerRoomSize-100.0)
				y = centerRoomY + 50.0 + simRng.Float64()*(centerRoomSize-100.0)
				if !collidesWithObstacleOrClosedExit(x, y) {
					break
				}
			}
		} else {
			for {
				x = 350.0 + simRng.Float64()*(WorldSize-700.0)
				y = 350.0 + simRng.Float64()*(WorldSize-700.0)
				if (x < centerRoomX || x > centerRoomX+centerRoomSize || y < centerRoomY || y > centerRoomY+centerRoomSize) &&
					!collidesWithObstacleOrClosedExit(x, y) {
					break
				}
			}
		}

		survivors[i] = Survivor{
			Alive:         true,
			Escaped:       false,
			ID:            int64(i),
			IsSurvivalist: simRng.Float64() < SurvivalistRatio,
			IsAlerted:     false,
			IsExhausted:   false,
			Stamina:       MaxStamina - int16(simRng.Intn(20)),
			X:             x,
			Y:             y,
			Speed:         2.4 + simRng.Float64()*0.8,
			Fear:          0.9 + simRng.Float64()*0.4,
			WanderAng:     simRng.Float64() * 2 * math.Pi,
		}
	}

	horde = make([]Zombie, InitialInfected)
	for k := range InitialInfected {
		survivors[k].Alive = false
		horde[k] = Zombie{
			Active:    true,
			ID:        int64(k),
			X:         survivors[k].X,
			Y:         survivors[k].Y,
			Speed:     2.3,
			WanderAng: simRng.Float64() * 2 * math.Pi,
		}
	}
}

func UpdateSimulation() {
	var sumX, sumY float64
	var aliveCount float64
	var aliveInRoomCount float64

	for i := 0; i < len(survivors); i++ {
		s := &survivors[i]
		if s.Alive && !s.Escaped {
			sumX += s.X
			sumY += s.Y
			aliveCount++
			if s.X >= centerRoomX && s.X <= centerRoomX+centerRoomSize && s.Y >= centerRoomY && s.Y <= centerRoomY+centerRoomSize {
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

	wasAlerted := make([]bool, len(survivors))
	for i := range survivors {
		wasAlerted[i] = survivors[i].IsAlerted
	}

	// 1. Détection & Alerte irréversible
	for i := 0; i < len(survivors); i++ {
		s := &survivors[i]
		if !s.Alive || s.Escaped || s.IsAlerted {
			continue
		}

		for zIdx := 0; zIdx < len(horde); zIdx++ {
			z := &horde[zIdx]
			if !z.Active {
				continue
			}
			dx := s.X - z.X
			dy := s.Y - z.Y
			if dx*dx+dy*dy <= AwarenessRadius*AwarenessRadius {
				s.IsAlerted = true
				break
			}
		}

		if !s.IsAlerted {
			for j := 0; j < len(survivors); j++ {
				if i == j || !wasAlerted[j] {
					continue
				}
				other := &survivors[j]
				if !other.Alive || other.Escaped {
					continue
				}

				dx := s.X - other.X
				dy := s.Y - other.Y
				if dx*dx+dy*dy <= PanicSpreadRadius*PanicSpreadRadius {
					if simRng.Float64() < PanicSpreadChance {
						s.IsAlerted = true
						break
					}
				}
			}
		}
	}

	// 2. Déplacement des survivants avec pénalité de fatigue lourde
	for i := 0; i < len(survivors); i++ {
		s := &survivors[i]
		if !s.Alive || s.Escaped {
			continue
		}

		for exIdx := 0; exIdx < len(exits); exIdx++ {
			ex := &exits[exIdx]
			if ex.IsClosed {
				continue
			}
			dx := ex.X - s.X
			dy := ex.Y - s.Y
			if math.Sqrt(dx*dx+dy*dy) <= ex.R {
				s.Escaped = true
				escaped++
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
			s.WanderAng += (simRng.Float64() - 0.5) * 0.2
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
			for zIdx := 0; zIdx < len(horde); zIdx++ {
				z := &horde[zIdx]
				if !z.Active {
					continue
				}
				zdx := s.X - z.X
				zdy := s.Y - z.Y
				zdist := math.Sqrt(zdx*zdx + zdy*zdy)
				if zdist < SurvivorRepelRadius && zdist > 0.001 {
					fleeX += (zdx / zdist)
					fleeY += (zdy / zdist)
					threatCount++
				}
			}

			if threatCount > 0 {
				flen := math.Sqrt(fleeX*fleeX + fleeY*fleeY)
				dx = (fleeX / flen) * s.Speed * 1.3
				dy = (fleeY / flen) * s.Speed * 1.3
			} else if s.IsSurvivalist {
				bestDist := math.MaxFloat64
				var targetExit *ExitZone
				for exIdx := 0; exIdx < len(exits); exIdx++ {
					ex := &exits[exIdx]
					if ex.IsClosed {
						continue
					}
					edx := ex.X - s.X
					edy := ex.Y - s.Y
					d := math.Sqrt(edx*edx + edy*edy)
					if d < bestDist {
						bestDist = d
						targetExit = ex
					}
				}

				if targetExit != nil {
					dx = ((targetExit.X - s.X) / bestDist) * s.Speed * s.Fear
					dy = ((targetExit.Y - s.Y) / bestDist) * s.Speed * s.Fear
				}
			} else {
				bestSurvDist := math.MaxFloat64
				var targetLeader *Survivor

				for j := 0; j < len(survivors); j++ {
					other := &survivors[j]
					if other.Alive && !other.Escaped && other.IsSurvivalist {
						sdx := other.X - s.X
						sdy := other.Y - s.Y
						d := math.Sqrt(sdx*sdx + sdy*sdy)
						if d < bestSurvDist {
							bestSurvDist = d
							targetLeader = other
						}
					}
				}

				if targetLeader != nil && bestSurvDist < 1800.0 {
					dx = ((targetLeader.X - s.X) / bestSurvDist) * s.Speed * s.Fear
					dy = ((targetLeader.Y - s.Y) / bestSurvDist) * s.Speed * s.Fear
				} else {
					s.WanderAng += (simRng.Float64() - 0.5) * 0.5
					dx = math.Cos(s.WanderAng) * s.Speed * 0.8
					dy = math.Sin(s.WanderAng) * s.Speed * 0.8
				}
			}
		}

		dx += (simRng.Float64() - 0.5) * 0.8
		dy += (simRng.Float64() - 0.5) * 0.8

		nextX := s.X + dx
		nextY := s.Y + dy

		if !collidesWithObstacleOrClosedExit(nextX, s.Y) {
			s.X = nextX
		}
		if !collidesWithObstacleOrClosedExit(s.X, nextY) {
			s.Y = nextY
		}
	}

	// 3. Traque des zombies : priorité à la sortie de la pièce si vide
	var newZombies []Zombie

	for i := 0; i < len(horde); i++ {
		z := &horde[i]
		if !z.Active {
			continue
		}

		for exIdx := 0; exIdx < len(exits); exIdx++ {
			ex := &exits[exIdx]
			if ex.HasSurvivalistIn && !ex.IsClosed {
				edx := ex.X - z.X
				edy := ex.Y - z.Y
				if math.Sqrt(edx*edx+edy*edy) <= ExitSealZombieRadius {
					ex.IsClosed = true
				}
			}
		}

		inRoom := (z.X >= centerRoomX && z.X <= centerRoomX+centerRoomSize && z.Y >= centerRoomY && z.Y <= centerRoomY+centerRoomSize)

		closestDist := math.MaxFloat64
		var target *Survivor

		for j := 0; j < len(survivors); j++ {
			s := &survivors[j]
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
				target = s
			}
		}

		dx, dy := 0.0, 0.0

		// Cas 1 : Cible humaine directe et visible
		if target != nil && closestDist < ZombieVisionRadius {
			dx = ((target.X - z.X) / closestDist) * z.Speed
			dy = ((target.Y - z.Y) / closestDist) * z.Speed

			if closestDist <= InfectionRadius {
				target.Alive = false
				newZombies = append(newZombies, Zombie{
					Active:    true,
					ID:        int64(len(horde) + len(newZombies)),
					X:         target.X,
					Y:         target.Y,
					Speed:     1.9 + simRng.Float64()*0.7,
					WanderAng: simRng.Float64() * 2 * math.Pi,
				})
			}
		} else if inRoom && aliveInRoomCount == 0 {
			// Cas 2 : Zombie enfermé dans une pièce dépeuplée -> cap direct sur la porte la plus proche
			targetDoorX := doorNorthX
			targetDoorY := doorNorthY
			if math.Abs(z.Y-doorSouthY) < math.Abs(z.Y-doorNorthY) {
				targetDoorX = doorSouthX
				targetDoorY = doorSouthY
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
				angleToCrowd := math.Atan2(cdy, cdx) + (simRng.Float64()-0.5)*0.6
				dx = math.Cos(angleToCrowd) * z.Speed * 0.9
				dy = math.Sin(angleToCrowd) * z.Speed * 0.9
			}
		} else {
			z.WanderAng += (simRng.Float64() - 0.5) * 0.4
			dx = math.Cos(z.WanderAng) * (z.Speed * 0.6)
			dy = math.Sin(z.WanderAng) * (z.Speed * 0.6)
		}

		nextX := z.X + dx
		nextY := z.Y + dy

		if !collidesWithObstacleOrClosedExit(nextX, z.Y) {
			z.X = nextX
		} else {
			z.WanderAng += math.Pi * 0.5
		}

		if !collidesWithObstacleOrClosedExit(z.X, nextY) {
			z.Y = nextY
		} else {
			z.WanderAng += math.Pi * 0.5
		}
	}

	if len(newZombies) > 0 {
		horde = append(horde, newZombies...)
	}
}

func generateHeatmap() []int {
	grid := make([]int, GridCols*GridRows)
	cellW := WorldSize / float64(GridCols)
	cellH := WorldSize / float64(GridRows)

	for _, z := range horde {
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

func getSurvivorCounts() (healthy, survs, normals, alerted, exhausted int) {
	for _, s := range survivors {
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

func getOpenExitsCount() int {
	c := 0
	for _, ex := range exits {
		if !ex.IsClosed {
			c++
		}
	}
	return c
}

func wsHandler(ws *websocket.Conn) {
	defer ws.Close()

	InitSimulation(MasterSeed)

	ticker := time.NewTicker(time.Second / time.Duration(TargetFPS))
	defer ticker.Stop()

	frames := 0
	lastCheck := time.Now()
	currentTPS := 0

	for range ticker.C {
		UpdateSimulation()

		frames++
		if time.Since(lastCheck) >= time.Second {
			currentTPS = frames
			frames = 0
			lastCheck = time.Now()
		}

		healthy, survCount, normCount, alertedCount, exhaustedCount := getSurvivorCounts()
		isFinished := (healthy == 0)

		survPoints := make([]Point, 0, healthy)
		for _, s := range survivors {
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

		processed := escaped + (len(horde) - InitialInfected)
		ratio := 0.0
		if processed > 0 {
			ratio = (float64(escaped) / float64(processed)) * 100.0
		}

		payload := FramePayload{
			TPS:       currentTPS,
			WorldSize: WorldSize,
			GridCols:  GridCols,
			GridRows:  GridRows,
			Stats: Stats{
				Seed:          MasterSeed,
				Infected:      len(horde),
				Healthy:       healthy,
				Alerted:       alertedCount,
				Exhausted:     exhaustedCount,
				Survivalists:  survCount,
				Normals:       normCount,
				Escaped:       escaped,
				OpenExits:     getOpenExitsCount(),
				SurvivalRatio: math.Round(ratio*10) / 10,
				IsFinished:    isFinished,
			},
			Obstacles: obstacles,
			Exits:     exits,
			Heatmap:   generateHeatmap(),
			Survivors: survPoints,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			break
		}

		if err := websocket.Message.Send(ws, string(data)); err != nil {
			break
		}
	}
}

func main() {
	http.Handle("/", http.FileServer(http.Dir(".")))
	http.Handle("/ws", websocket.Handler(wsHandler))

	port := 8080
	fmt.Printf("Moteur d'épidémie optimisable sur http://localhost:%d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
