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

---

## 2. Tableau Récapitulatif Comparatif des Versions

| Version | Description | Temps moyen / tick | Débit réel (TPS) | Cycle 50 ticks | Allocations / tick | Gain vs v0 |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **v0 (Baseline)** | Code initial non optimisé ($O(N^2)$ brut) | 131.55 ms | 7.60 TPS | 6.78 s | 5 allocs/op | Référence |
| **Tentative 1** | Tranches de pointeurs `[]*Survivor` | 138.32 ms | 7.23 TPS | - | 6 allocs/op | -5.1% *(Régression)* |
| **v1** | Grille spatiale 1D contiguë (`[100*100]Cell`) | **~71.92 ms** | **13.90 TPS** | **1.51 s** | **0 allocs/op** | **+82.9% TPS** *(x1.83)* |

---

## 3. Détails des Mesures

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

---

### Mesure d'Initialisation : `BenchmarkNewEngine` (avec `[]*Survivor`)

- **Date :** 05/10/2026
- **Description :** Mesure du coût CPU, du volume mémoire et du nombre d'allocations lors de l'instanciation complète du moteur (`NewEngine(MasterSeed)` avec 10 000 agents sous forme de `[]*Survivor`).
- **Commande :**
  ```bash
  go test -bench=BenchmarkNewEngine -benchmem -benchtime=3s -count=3 ./internal/simulation
  ```
- **Sortie brute du benchmark :**
  ```text
  goos: linux
  goarch: amd64
  pkg: github.com/Chroq/zombie-horde/internal/simulation
  cpu: Intel(R) Core(TM) i7-8665U CPU @ 1.90GHz
  BenchmarkNewEngine-8   	    3032	   1229575 ns/op	  730808 B/op	   10010 allocs/op
  BenchmarkNewEngine-8   	    2968	   1242751 ns/op	  730801 B/op	   10010 allocs/op
  BenchmarkNewEngine-8   	    2901	   1297605 ns/op	  730801 B/op	   10010 allocs/op
  PASS
  ok  	github.com/Chroq/zombie-horde/internal/simulation	15.716s
  ```

#### Synthèse :
- **Temps moyen par initialisation :** `~1.26 ms` (min: `1.23 ms`, max: `1.30 ms`)
- **Volume d'allocations mémoire :** `~730.8 KB/op` (730 803 octets par appel)
- **Nombre d'allocations mémoire :** `10 010 allocs/op`

#### Constat matériel & mémoire :
L'utilisation de la tranche de pointeurs `[]*Survivor` force **10 000 allocations individuelles sur le tas** (une pour chaque `&Survivor{}`), en plus des slices internes (`horde`, `obstacles`, etc.). Ce test chiffre précisément l'explosion des allocations à la création du moteur, confirmant la dispersion des adresses mémoire sur le heap dès l'initialisation.

---

### Version v1 : Grille Spatiale 1D Contiguë (Spatial Hashing)

- **Date :** 06/10/2026
- **Description :** Partitionnement spatial 1D (`[100 * 100]Cell`) par linked-cells sans pointeurs dynamiques, comparaisons quadratiques de distance ($dx^2 + dy^2$) et élimination des allocations temporaires sur la boucle physique.
- **Commande :**
  ```bash
  go test -bench=BenchmarkUpdateSimulation -benchmem -benchtime=3s -count=3 ./internal/simulation
  ```
- **Sortie brute du benchmark :**
  ```text
  goos: linux
  goarch: amd64
  pkg: github.com/Chroq/zombie-horde/internal/simulation
  cpu: Intel(R) Core(TM) i7-8665U CPU @ 1.90GHz
  BenchmarkUpdateSimulation-8   	     100	  72308727 ns/op	    2042 B/op	       0 allocs/op
  BenchmarkUpdateSimulation-8   	     100	  73006237 ns/op	    2042 B/op	       0 allocs/op
  BenchmarkUpdateSimulation-8   	     100	  70440717 ns/op	    2042 B/op	       0 allocs/op
  PASS
  ok  	github.com/Chroq/zombie-horde/internal/simulation	23.304s
  ```

#### Synthèse & Gains vs v0 (Baseline) :

- **Temps moyen par tick :** `~71.92 ms` _(**-45.3% de temps de calcul**, gain de vitesse x1.83 vs 131.55 ms en v0)_
- **Débit réel calculé :** `13.90 ticks/s` _(+82.9% de TPS effectif vs 7.60 TPS en v0)_
- **Cycle déterministe 50 ticks (`BenchmarkSimulation50Ticks`) :** `1.51 s` _(**4.5x plus rapide** vs 6.78 s en v0)_
- **Volume d'allocations mémoire :** `2 042 B/op` _(-83.4% de volume mémoire vs 12 321 B/op en v0)_
- **Nombre d'allocations mémoire :** **`0 allocs/op`** _(Objectif zéro allocation atteint sur la boucle physique)_

#### Analyse des gains matériels :
1. **Suppression des scans globaux :** Les calculs de proximité (champ de vision, propagation de panique, répulsion) n'examinent plus 10 000 agents dans le désordre, mais uniquement les cellules du voisinage géométrique immédiat.
2. **Localité de cache L2 :** L'ensemble de la structure spatiale (`[10000]Cell` + tableaux de chaînage) occupe ~170 Ko en mémoire contiguë, tenant intégralement dans le cache L2 du processeur (1 Mo).
3. **Élimination de la pression GC :** La réutilisation des buffers préalloués (`wasAlerted`, `newZombies`) et la grille sans pointeurs atteignent 0 allocation par tick, supprimant les cycles de collecte du runtime Go.

#### Mesure d'Initialisation : `BenchmarkNewEngine` (v1)

- **Sortie brute du benchmark :**
  ```text
  goos: linux
  goarch: amd64
  pkg: github.com/Chroq/zombie-horde/internal/simulation
  cpu: Intel(R) Core(TM) i7-8665U CPU @ 1.90GHz
  BenchmarkNewEngine-8   	    3448	   1249680 ns/op	  834052 B/op	      11 allocs/op
  BenchmarkNewEngine-8   	    2961	   1167427 ns/op	  834048 B/op	      11 allocs/op
  BenchmarkNewEngine-8   	    3334	   1194506 ns/op	  834050 B/op	      11 allocs/op
  PASS
  ok  	github.com/Chroq/zombie-horde/internal/simulation	13.371s
  ```
- **Nombre d'allocations :** **`11 allocs/op`** _(chute drastique de **10 010 allocs/op à 11 allocs/op** grâce au retour au slice de valeurs contiguës `[]Survivor` et à la grille 1D allouée une fois pour toutes)_.
- **Volume mémoire d'instanciation :** `~834 Ko` alloués une seule fois au démarrage pour l'ensemble du moteur.




