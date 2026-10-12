package simulation_test

import (
	"bytes"
	"runtime"
	"testing"
	"time"

	"github.com/Chroq/zombie-horde/internal/simulation"
)

func BenchmarkNewEngine(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		simulation.NewEngine(simulation.MasterSeed)
	}
}

func BenchmarkUpdateSimulation(b *testing.B) {
	b.ReportAllocs()
	engine := simulation.NewEngine(simulation.MasterSeed)

	for b.Loop() {
		engine.Update()
	}
}

func BenchmarkSimulation50Ticks(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		b.StopTimer()
		engine := simulation.NewEngine(simulation.MasterSeed)
		b.StartTimer()

		for range 50 {
			engine.Update()
		}
	}
}

func BenchmarkGenerateHeatmap(b *testing.B) {
	b.ReportAllocs()
	engine := simulation.NewEngine(simulation.MasterSeed)

	for b.Loop() {
		engine.GenerateHeatmap()
	}
}

func BenchmarkBuildFramePayload(b *testing.B) {
	b.ReportAllocs()
	engine := simulation.NewEngine(simulation.MasterSeed)

	for b.Loop() {
		engine.BuildFramePayload(30)
	}
}

func TestDeterministicRun(t *testing.T) {
	e1 := simulation.NewEngine(simulation.MasterSeed)
	e2 := simulation.NewEngine(simulation.MasterSeed)

	for range 10 {
		e1.Update()
		e2.Update()
	}

	h1, _, _, a1, _ := e1.GetSurvivorCounts()
	h2, _, _, a2, _ := e2.GetSurvivorCounts()

	if h1 != h2 || a1 != a2 {
		t.Fatalf("Simulation non déterministe: (%d, %d) vs (%d, %d)", h1, a1, h2, a2)
	}
}

// TestInfectionSpread vérifie que les zombies contaminent effectivement les humains
// et que la horde s'agrandit au fil des ticks, augmentant les stats et la heatmap.
func TestInfectionSpread(t *testing.T) {
	engine := simulation.NewEngine(simulation.MasterSeed)

	initialPayload := engine.BuildFramePayload(0)
	if initialPayload.Stats.Infected != simulation.InitialInfected {
		t.Fatalf("Infectés initiaux attendus: %d, obtenu: %d", simulation.InitialInfected, initialPayload.Stats.Infected)
	}

	// Avancement de 30 ticks
	for range 30 {
		engine.Update()
	}

	payload := engine.BuildFramePayload(30)

	// Vérification de la propagation de l'infection
	if payload.Stats.Infected <= simulation.InitialInfected {
		t.Fatalf("Échec d'infection: aucun nouveau zombie créé après 30 ticks (infectés=%d)", payload.Stats.Infected)
	}

	expectedSurvivors := simulation.TotalSurvivors - payload.Stats.Infected - payload.Stats.Escaped
	if payload.Stats.Healthy != expectedSurvivors {
		t.Fatalf("Incohérence des survivants sains: attendu %d, obtenu %d", expectedSurvivors, payload.Stats.Healthy)
	}

	// Vérification de la heatmap (les zombies doivent y être visibles)
	totalInHeatmap := 0
	for _, count := range payload.Heatmap {
		totalInHeatmap += count
	}

	if totalInHeatmap != payload.Stats.Infected {
		t.Fatalf("Heatmap incohérente: somme=%d vs horde=%d", totalInHeatmap, payload.Stats.Infected)
	}

	t.Logf("Progression après 30 ticks: %d infectés (initial: %d), %d rescapés, %d sains restants",
		payload.Stats.Infected, simulation.InitialInfected, payload.Stats.Escaped, payload.Stats.Healthy)
}

// TestSurvivorChunkPartitioning vérifie que la population de 10 000 survivants est
// scindée de façon strictement équitable, contiguë et sans chevauchement en sous-tranches
// indépendantes pour les workers (ex. 8 tranches de 1 250 entités).
func TestSurvivorChunkPartitioning(t *testing.T) {
	engine := simulation.NewEngine(simulation.MasterSeed)
	chunks := engine.SurvivorChunks()

	if len(chunks) != simulation.WorkerCount {
		t.Fatalf("Nombre de chunks attendu: %d, obtenu: %d", simulation.WorkerCount, len(chunks))
	}

	totalEntities := 0
	expectedPerChunk := simulation.TotalSurvivors / simulation.WorkerCount

	for i, chunk := range chunks {
		if chunk.WorkerID != i {
			t.Errorf("Chunk %d: WorkerID attendu %d, obtenu %d", i, i, chunk.WorkerID)
		}

		chunkSize := chunk.Count()
		if chunkSize != expectedPerChunk {
			t.Errorf("Chunk %d: taille attendue %d, obtenue %d", i, expectedPerChunk, chunkSize)
		}

		if i == 0 && chunk.Start != 0 {
			t.Errorf("Premier chunk doit commencer à 0, commence à %d", chunk.Start)
		}

		if i > 0 && chunk.Start != chunks[i-1].End {
			t.Errorf("Discontinuité entre chunk %d et %d: %d != %d", i-1, i, chunks[i-1].End, chunk.Start)
		}

		slice := engine.SurvivorSlice(chunk)
		if len(slice) != chunkSize {
			t.Errorf("Chunk %d: sous-tranche directe a une longueur de %d au lieu de %d", i, len(slice), chunkSize)
		}

		totalEntities += chunkSize
	}

	if totalEntities != simulation.TotalSurvivors {
		t.Fatalf("Total des entités partitionnées: %d, attendu: %d", totalEntities, simulation.TotalSurvivors)
	}

	lastChunk := chunks[len(chunks)-1]
	if lastChunk.End != simulation.TotalSurvivors {
		t.Fatalf("Dernier chunk se termine à %d, attendu %d", lastChunk.End, simulation.TotalSurvivors)
	}
}

// TestSurvivorChunkPartitioningScaling vérifie l'équité du découpage pour différentes tailles de pool (1, 2, 4, 8, 16 workers).
func TestSurvivorChunkPartitioningScaling(t *testing.T) {
	workerConfigurations := []int{1, 2, 4, 8, 16}

	for _, numWorkers := range workerConfigurations {
		eng := simulation.NewEngineWithWorkers(simulation.MasterSeed, numWorkers)
		chunks := eng.SurvivorChunks()

		if len(chunks) != numWorkers {
			t.Fatalf("Pour %d workers, obtenu %d chunks", numWorkers, len(chunks))
		}

		covered := 0
		for i, c := range chunks {
			if i > 0 && c.Start != chunks[i-1].End {
				t.Fatalf("Workers=%d, chevauchement/trou au chunk %d: start=%d prevEnd=%d", numWorkers, i, c.Start, chunks[i-1].End)
			}
			covered += c.Count()
		}

		if covered != simulation.TotalSurvivors {
			t.Fatalf("Workers=%d, entités totales couvertes: %d != %d", numWorkers, covered, simulation.TotalSurvivors)
		}
	}
}

// TestWorkerPoolBoundedToNumCPU vérifie que le Worker Pool est strictement borné
// sur le nombre de cœurs de la machine hôte (runtime.NumCPU()).
func TestWorkerPoolBoundedToNumCPU(t *testing.T) {
	eng := simulation.NewEngine(simulation.MasterSeed)
	defer eng.Close()

	if eng.WorkerCount() != runtime.NumCPU() {
		t.Fatalf("Taille du Worker Pool attendue (%d cœurs), obtenue: %d", runtime.NumCPU(), eng.WorkerCount())
	}

	chunks := eng.SurvivorChunks()
	if len(chunks) != runtime.NumCPU() {
		t.Fatalf("Nombre de chunks attendu (%d), obtenu: %d", runtime.NumCPU(), len(chunks))
	}
}

// TestWorkerPoolEarlyCancellation vérifie que la fermeture du moteur propage
// l'annulation précoce et stoppe les workers proprement sans deadlock.
func TestWorkerPoolEarlyCancellation(t *testing.T) {
	eng := simulation.NewEngine(simulation.MasterSeed)

	// Exécution de quelques ticks
	for range 3 {
		eng.Update()
	}

	done := make(chan struct{})
	go func() {
		eng.Close()
		close(done)
	}()

	select {
	case <-done:
		// Succès : arrêt immédiat
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Deadlock lors de l'arrêt précoce du Worker Pool via context.Cancel")
	}
}

// TestSyncPoolPointSliceRecycling vérifie que le PointSlicePool recycle correctement les tampons de télémétrie.
func TestSyncPoolPointSliceRecycling(t *testing.T) {
	slice1 := simulation.GetPointSlice()
	if slice1 == nil || len(*slice1) != 0 || cap(*slice1) < simulation.TotalSurvivors {
		t.Fatalf("Tranche initiale invalide: len=%d, cap=%d", len(*slice1), cap(*slice1))
	}

	*slice1 = append(*slice1, simulation.Point{X: 10, Y: 20})
	simulation.PutPointSlice(slice1)

	slice2 := simulation.GetPointSlice()
	if len(*slice2) != 0 {
		t.Fatalf("La tranche recyclée doit être réinitialisée (len=0), obtenu: %d", len(*slice2))
	}
	simulation.PutPointSlice(slice2)
}

// BenchmarkBuildFramePayloadWithPool mesure le coût de construction avec recyclage sync.Pool.
func BenchmarkBuildFramePayloadWithPool(b *testing.B) {
	b.ReportAllocs()
	engine := simulation.NewEngine(simulation.MasterSeed)
	defer engine.Close()

	for b.Loop() {
		_, slice := engine.BuildFramePayloadWithPool(30)
		simulation.PutPointSlice(slice)
	}
}

// BenchmarkMetricsContentionMutex simule la mise à jour concurrente des compteurs sous sync.Mutex.
func BenchmarkMetricsContentionMutex(b *testing.B) {
	b.ReportAllocs()
	runtime.SetMutexProfileFraction(1)
	engine := simulation.NewEngine(simulation.MasterSeed)
	defer engine.Close()
	engine.SetUseMutex(true)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			engine.RecordEscape()
			engine.RecordInfection()
		}
	})
}

// BenchmarkMetricsContentionAtomic simule la mise à jour concurrente des compteurs via sync/atomic (lock-free).
func BenchmarkMetricsContentionAtomic(b *testing.B) {
	b.ReportAllocs()
	runtime.SetMutexProfileFraction(1)
	engine := simulation.NewEngine(simulation.MasterSeed)
	defer engine.Close()
	engine.SetUseMutex(false)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			engine.RecordEscape()
			engine.RecordInfection()
		}
	})
}

// TestObstacleGridCollisions valide la détection de collision sur les parois et dans les obstacles.
func TestObstacleGridCollisions(t *testing.T) {
	engine := simulation.NewEngine(simulation.MasterSeed)
	defer engine.Close()

	// Les limites du monde doivent toujours être détectées comme collision
	if !engine.CollidesWithObstacle(10, 2500) {
		t.Fatalf("La limite gauche x=10 aurait dû être en collision")
	}
	if !engine.CollidesWithObstacle(2500, 10) {
		t.Fatalf("La limite haute y=10 aurait dû être en collision")
	}
	if !engine.CollidesWithObstacle(simulation.WorldSize-10, 2500) {
		t.Fatalf("La limite droite x=%f aurait dû être en collision", simulation.WorldSize-10)
	}
}

// BenchmarkObstacleCollisionGrid mesure la rapidité du test de collision O(1) via la grille d'obstacles.
func BenchmarkObstacleCollisionGrid(b *testing.B) {
	b.ReportAllocs()
	engine := simulation.NewEngine(simulation.MasterSeed)
	defer engine.Close()

	coords := [8][2]float64{
		{2500, 2500},
		{100, 100},
		{4900, 4900},
		{1250, 3750},
		{3750, 1250},
		{2050, 2050},
		{2950, 2950},
		{500, 4500},
	}

	b.ResetTimer()
	for b.Loop() {
		for _, c := range coords {
			_ = engine.CollidesWithObstacle(c[0], c[1])
		}
	}
}

// TestBinaryEncoding valide la sérialisation des frames binaires Init et Tick.
func TestBinaryEncoding(t *testing.T) {
	engine := simulation.NewEngine(simulation.MasterSeed)
	defer engine.Close()

	var buf bytes.Buffer
	engine.EncodeInitFrame(&buf)

	if buf.Len() == 0 || buf.Bytes()[0] != simulation.FrameTypeInit {
		t.Fatalf("Init frame invalide: len=%d, type=%d", buf.Len(), buf.Bytes()[0])
	}

	buf.Reset()
	isFinished := engine.EncodeTickFrame(&buf, 30)

	if isFinished {
		t.Fatalf("La simulation ne doit pas être terminée au premier tick")
	}

	if buf.Len() == 0 || buf.Bytes()[0] != simulation.FrameTypeTick {
		t.Fatalf("Tick frame invalide: len=%d, type=%d", buf.Len(), buf.Bytes()[0])
	}

	// La frame binaire pour 10 000 agents vivants doit mesurer environ 50 Ko (< 55 Ko)
	if buf.Len() > 55000 {
		t.Fatalf("Frame binaire trop volumineuse: %d octets (attendu < 55 Ko)", buf.Len())
	}
}

// BenchmarkEncodeBinaryFrame mesure le débit et le coût CPU de sérialisation binaire directe.
func BenchmarkEncodeBinaryFrame(b *testing.B) {
	b.ReportAllocs()
	engine := simulation.NewEngine(simulation.MasterSeed)
	defer engine.Close()

	buf := bytes.NewBuffer(make([]byte, 0, 64*1024))

	b.ResetTimer()
	for b.Loop() {
		engine.EncodeTickFrame(buf, 30)
	}
}
