package simulation_test

import (
	"testing"

	"github.com/Chroq/zombie-horde/internal/simulation"
)

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
