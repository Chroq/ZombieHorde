# Suivi des Performances & Benchmarks

## 1. Environnement de Test

- **Date :** 05 octobre 2026
- **Système d'exploitation :** Linux EndeavourOS (x86_64)
- **Processeur (CPU) :** Intel(R) Core(TM) i7-8665U CPU @ 1.90GHz (4 cœurs, 8 threads)
- **Hiérarchie de Caches :**
  - **L1d :** 128 KiB (4 x 32 KiB)
  - **L1i :** 128 KiB (4 x 32 KiB)
  - **L2 :** 1 MiB (4 x 256 KiB)
  - **L3 :** 8 MiB (partagé)
- **Mémoire vive (RAM) :** 16 Go
- **Version Go :** `go1.27.1-X:nodwarf5 linux/amd64`
- **Commande de benchmark :**
  ```bash
  go test -bench=BenchmarkUpdateSimulation -benchmem -benchtime=3s -count=3
  ```

## 2. Détails des Mesures

### Version v0 (Baseline - Code Initial)

- **Date :** 05/10/2026
- **Description :** Version initiale non optimisée du moteur (`main.go` / `UpdateSimulation`).
- **Sortie brute du benchmark :**
  ```text
  goos: linux
  goarch: amd64
  pkg: github.com/Chroq/zombie-horde
  cpu: Intel(R) Core(TM) i7-8665U CPU @ 1.90GHz
  BenchmarkUpdateSimulation-8   	      44	 122125030 ns/op	   11886 B/op	       5 allocs/op
  BenchmarkUpdateSimulation-8   	      49	 125613393 ns/op	   12539 B/op	       5 allocs/op
  BenchmarkUpdateSimulation-8   	      49	 146925989 ns/op	   12539 B/op	       5 allocs/op
  PASS
  ok  	github.com/Chroq/zombie-horde	18.739s
  ```

#### Synthèse v0 :

- **Temps moyen par tick :** `131.55 ms` (min: `122.13 ms`, max: `146.93 ms`)
- **Débit réel calculé :** `7.60 ticks/s` _(inférieur à la cible temps réel de 30 TPS, le serveur n'arrive pas à tenir la cadence)_
- **Volume d'allocations mémoire :** `~12.32 KB/op` (12 321 octets par tick)
- **Nombre d'allocations mémoire :** `5 allocs/op`

---

### Tentative 1 : Tranches de Pointeurs (`[]*Survivor`) — Échec constructif

- **Date :** 05/10/2026
- **Description :** Remplacement de la tranche de structures contiguës `[]Survivor` par une tranche de pointeurs `[]*Survivor` dans `Engine`.
- **Sortie brute du benchmark :**
  ```text
  goos: linux
  goarch: amd64
  pkg: github.com/Chroq/zombie-horde
  cpu: Intel(R) Core(TM) i7-8665U CPU @ 1.90GHz
  BenchmarkUpdateSimulation-8   	      75	 139169538 ns/op	   13944 B/op	       6 allocs/op
  BenchmarkUpdateSimulation-8   	      73	 137868802 ns/op	   13962 B/op	       6 allocs/op
  BenchmarkUpdateSimulation-8   	      75	 137909855 ns/op	   13944 B/op	       6 allocs/op
  PASS
  ok  	github.com/Chroq/zombie-horde	33.035s
  ```

#### Synthèse & Comparaison vs v0 :

- **Temps moyen par tick :** `138.32 ms` _(**+5.1% de régression** vs 131.55 ms en v0)_
- **Débit réel calculé :** `7.23 ticks/s` _(baisse de débit vs 7.60 TPS en v0)_
- **Volume d'allocations mémoire :** `~13.95 KB/op` (13 950 octets par tick, soit +1 629 octets/tick)
- **Nombre d'allocations mémoire :** `6 allocs/op` _(+1 allocation par tick)_

#### Analyse matérielle du goulot :
1. **Perte de localité spatiale (Cache Misses L1/L2) :** `[]Survivor` garantissait un alignement contigu en RAM, permettant au Hardware Prefetcher du CPU d'anticiper le chargement par lignes de cache de 64 octets. La tranche de pointeurs `[]*Survivor` force un déréférencement systématique (pointer chasing) vers des adresses dispersées sur le tas.
2. **Pression accrue sur le Garbage Collector :** Le graphe d'objets contient 10 000 pointeurs individuels supplémentaires à scanner lors des phases de marquage du runtime Go, augmentant également le volume d'allocation par tick.

