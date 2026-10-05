package simulation

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
