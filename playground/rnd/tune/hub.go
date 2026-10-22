package main

const (
	tuneSeedHi = 0x7C0FFEE0

	nInit         = 8
	candidatePool = 512
	perturbPool   = 64
	perturbScale  = 0.1
	acqXi         = 0.01

	gpSigmaF        = 1.0
	gpSigmaN        = 1e-6
	mapSigmaN       = 1e-2
	mapSteps        = 8
	mapStepSize     = 0.4
	mapH            = 1e-3
	priorMeanFrac   = 0.3
	priorSd         = 1.0
	lengthFloorFrac = 1e-3
	lengthCeilFrac  = 10.0
	choleskyFloor   = 1e-12

	gridLevels = 13

	acoAnts        = 8
	acoLevels      = 13
	acoEvap        = 0.25
	acoDeposit     = 1.0
	acoGlobalBoost = 2.0
	acoPherFloor   = 0.1

	seedCount          = 20
	tableBudget        = 120
	tableWorkers       = 8
	validationSeedBase = 0xB055
	noiseSeedBase      = 0x0BADC0DE

	noiseSigma = 0.05

	braninLo0      = -5.0
	braninHi0      = 10.0
	braninLo1      = 0.0
	braninHi1      = 15.0
	braninTarget   = 0.498
	hartmannLo     = 0.0
	hartmannHi     = 1.0
	hartmannTarget = -3.7628
	quadDomain     = 5.0
	quadTarget     = 0.3
	quadCenter0    = 1.7
	quadCenter1    = -2.4
	quadCenter2    = 0.9

	demoTcIdx           = 3
	demoBudget          = 40
	demoKpLo            = 0.01
	demoKpHi            = 0.06
	demoKiLo            = 0.005
	demoKiHi            = 0.03
	demoKdStep          = 0.005
	demoKdLevels        = 3
	demoOvLo            = 1.0
	demoOvHi            = 1.3
	demoSlowStartMoves  = 10
	demoSlowStartFactor = 3.0
	demoSeedHi          = 0xC10C7EED
	demoSeedLo          = 0x7E11
)
