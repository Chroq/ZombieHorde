package main_test

import (
	"testing"

	"github.com/Chroq/zombie-horde/internal/simulation"
)

// BenchmarkUpdateSimulation mesure le coût d'exécution d'un tick unitaire (UpdateSimulation).
// b.ReportAllocs() garantit que les métriques d'allocation (B/op et allocs/op) sont systématiquement tracées.
func BenchmarkUpdateSimulation(b *testing.B) {
	b.ReportAllocs()
	simulation.InitSimulation(simulation.MasterSeed)

	for b.Loop() {
		simulation.UpdateSimulation()
	}
}

// BenchmarkSimulation50Ticks exécute un scénario déterministe standardisé de 50 ticks
// réinitialisé à chaque itération (MasterSeed = 42).
// Cela garantit une charge de calcul strictement identique et comparable entre toutes les versions (v0, v1, v2...).
func BenchmarkSimulation50Ticks(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		b.StopTimer()
		simulation.InitSimulation(simulation.MasterSeed)
		b.StartTimer()

		for range 50 {
			simulation.UpdateSimulation()
		}
	}
}

func TestSimulationDeterministicRun(t *testing.T) {
	simulation.InitSimulation(simulation.MasterSeed)
	for range 10 {
		simulation.UpdateSimulation()
	}
}
