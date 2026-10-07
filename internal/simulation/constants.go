package simulation

const (
	MasterSeed             int64   = 42
	WorldSize              float64 = 5000.0
	GridCols               int     = 100
	GridRows                       = GridCols
	TotalSurvivors         int     = 10000
	CenterRoomSurvivors    int     = 1500
	InitialInfected        int     = 5
	SurvivalistRatio       float64 = 0.12
	ObstacleCount          int     = 35
	ExitRadius             float64 = 75.0
	ExitRadiusSq           float64 = ExitRadius * ExitRadius
	TargetFPS              int     = 30
	InfectionRadius        float64 = 22.0
	InfectionRadiusSq      float64 = InfectionRadius * InfectionRadius
	ZombieVisionRadius     float64 = 1600.0
	ZombieVisionRadiusSq   float64 = ZombieVisionRadius * ZombieVisionRadius
	SurvivorRepelRadius    float64 = 350.0
	SurvivorRepelRadiusSq  float64 = SurvivorRepelRadius * SurvivorRepelRadius
	ExitSealZombieRadius   float64 = 250.0
	ExitSealZombieRadiusSq float64 = ExitSealZombieRadius * ExitSealZombieRadius
	AwarenessRadius        float64 = 160.0
	AwarenessRadiusSq      float64 = AwarenessRadius * AwarenessRadius
	PanicSpreadRadius      float64 = 70.0
	PanicSpreadRadiusSq    float64 = PanicSpreadRadius * PanicSpreadRadius
	PanicSpreadChance      float64 = 0.05

	// Pénalité de fatigue sévère
	MaxStamina          int16 = 100
	StaminaDrainSprint  int16 = 2  // Épuisement deux fois plus rapide en panique
	StaminaRecoveryRest int16 = 1  // Récupération lente au repos
	StaminaResumeSprint int16 = 60 // Immobilisation prolongée (~2 secondes sans bouger)
)
