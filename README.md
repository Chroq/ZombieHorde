# ZombieHorde • Simulateur d'Épidémie 2D

Moteur de simulation spatiale en temps réel modélisant la propagation d'une infection au sein d'une population d'agents autonomes dans un environnement fermé avec obstacles. L'architecture repose sur un serveur de calcul écrit en Go et une interface de télémétrie web servie via WebSocket.

---

## 1. Spécifications & Règles Métier

- **Carte et Environnement :**
  - Espace torique/borné de 5 000 x 5 000 unités entouré de murs infranchissables.
  - Pièce centrale (« Foyer Zéro ») munie de deux accès étroits (Nord et Sud).
  - Labyrinthe procédural composé d'obstacles rectangulaires infranchissables.
  - 4 sas d'évacuation fixes situés aux extrémités de la carte.

- **Comportement des Entités :**
  - **Horde (Infectés) :** Les zombies pourchassent les survivants dans leur champ de vision direct. S'ils sont enfermés dans la pièce centrale vide, ils ciblent en priorité les portes pour sortir. À défaut de cible visible, ils convergent vers le centre de gravité de la foule humaine restante.
  - **Contamination :** Tout humain entrant en contact avec un zombie est instantanément infecté et rejoint la horde.
  - **Survivalistes (12 % de la population) :** Connaissent l'emplacement des sas d'évacuation et s'y dirigent directement dès qu'ils sont alertés.
  - **Civils Normaux (88 % de la population) :** Ignorent la position des issues. En cas d'alerte, ils fuient la horde et tentent de suivre le survivaliste le plus proche. En l'absence de guide, ils errent en panique.
  - **Alerte & Panique irréversible :** Un survivant s'alerte s'il aperçoit un zombie ou par contagion si un voisin en panique passe à proximité. L'état d'alerte est définitif.
  - **Endurance & Fatigue :** Courir en état d'alerte consomme de l'endurance. Une fois la jauge épuisée, l'humain tombe en état d'épuisement et reste totalement immobile pendant plusieurs secondes pour reprendre son souffle.
  - **Verrouillage des Sas :** Dès qu'un survivaliste pénètre dans un sas, celui-ci s'arme. Si un zombie s'approche à moins de 250 unités de ce sas, la porte est scellée définitivement, devenant un obstacle physique infranchissable pour les autres survivants.

- **Déterminisme :**
  - Utilisation d'une graine pseudo-aléatoire fixe (MasterSeed = 42) garantissant la stricte reproductibilité des trajectoires et des statistiques à chaque lancement.

---

## 2. Structure du Projet

```text
.
├── cmd/
│   └── server/          # Point d'entrée de l'application serveur
│       └── main.go
├── internal/
│   ├── server/          # Serveur HTTP et WebSocket de télémétrie
│   │   └── server.go
│   └── simulation/      # Moteur de simulation spatiale et règles métier
│       ├── constants.go
│       ├── engine.go
│       ├── simulation_test.go
│       └── types.go
├── index.html           # Interface web autonome (Canvas 2D, télémétrie, dashboard)
├── go.mod               # Déclaration du module Go
├── go.sum               # Sommes de contrôle des dépendances
├── main.go              # Point d'entrée racine (raccourci)
└── main_test.go         # Benchmarks et tests de performance racine
```

## 3. Prérequis

- Go : version 1.20 ou supérieure.
- Un navigateur web moderne compatible WebSocket et Canvas HTML5 (Chrome, Firefox, Safari, Edge).

## 4. Installation & Lancement

Télécharger les dépendances :

```bash
go mod tidy
```

Démarrer le serveur de simulation :

```bash
go run main.go
```

Accéder à l'interface de contrôle :

```bash
http://localhost:8080
```

## 5. Indicateurs de Télémétrie

Le panneau latéral restitue en temps réel l'état du système :IndicateurDescriptionSeed DéterministeGraine utilisée pour l'initialisation du générateur pseudo-aléatoire (42).TPS (Ticks Per Second)Fréquence réelle de mise à jour de la boucle de calcul serveur (cible : 30 TPS).FPS (Frames Per Second)Taux de rafraîchissement graphique rendu par le moteur Canvas du navigateur.Taux de SurviePourcentage d'humains ayant réussi à évacuer par un sas ouvert par rapport au total des pertes.Panique & ÉpuisementNombre d'agents alertés vs agents temporairement immobilisés par fatigue.Sas OuvertsNombre de sas d'évacuation encore praticables sur les 4 initiaux.Effectifs en VieRépartition des survivants actifs (Survivalistes / Civils).Horde / RescapésNombre total d'infectés en jeu vs nombre total d'évacués.

---

### Astuces supplémentaires si la copie pose encore problème :

- **Utiliser l'icône de copie du bloc :** Clique directement sur le petit bouton de copie en haut à droite du cadre de code ci-dessus, cela copie le texte brut dans le presse-papier sans passer par la sélection visuelle de la souris.
- **Affichage source navigateur :** Si ton interface web interprète malgré tout le bloc, fais un clic droit sur le texte > **Inspecter l'élément** (ou `F12`), puis copie directement le contenu de la balise `<pre><code>`.
