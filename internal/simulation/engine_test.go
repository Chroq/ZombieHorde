package simulation_test

import (
	"testing"

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
