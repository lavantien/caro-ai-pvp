package main

const (
	dePop        = 48
	deGens       = 400
	deF          = 0.7
	deCR         = 0.9
	smoothBudget = 20000

	gamePop           = 16
	gameGens          = 2000
	gameBudget        = 200000
	gamesPerCandidate = 20
	swissOpponents    = 4

	sprtP1       = 0.58
	sprtAlpha    = 0.05
	sprtBeta     = 0.05
	sprtMaxGames = 64
	btIters      = 50
	btStep       = 0.1

	shadeMemSize   = 6
	shadePBestFrac = 0.15
	shadeNMin      = 4
	shadeStallGens = 25
	shadeDivEps    = 1e-6

	screenWarmup   = 200
	screenLambda   = 1e-6
	screenQuantile = 0.75
	screenSkipCap  = 0.9

	seeds = 21

	rastriginTarget  = 1e-6
	rosenbrockTarget = 1.0
	gameTargetEps    = 0.05
	marginWeak       = 0.12266826044534127
	marginMid        = 0.49067304178136528
	marginStrong     = 1.0099496221211108
	clampPMin        = 0.02
	clampPMax        = 0.98
)

type cfg struct {
	Name      string
	Shade     bool
	Estimator string
	CRN       bool
	Surrogate bool
	Repair    bool
	Pop       int
	Gens      int
	Budget    int
}

func smoothBaseCfg(name string) cfg {
	return cfg{Name: name, Pop: dePop, Gens: deGens, Budget: smoothBudget}
}

func gameCfg(name string) cfg {
	return cfg{Name: name, Shade: true, Estimator: "ref", Pop: gamePop, Gens: gameGens, Budget: gameBudget}
}

func isRankMode(c cfg) bool {
	return c.Estimator == "bt" || c.Estimator == "btsprt"
}
