package main

const (
	arenaDirOut     = "playground/arena/out"
	arenaDirArchive = "logs/archive"
	arenaDirScratch = ".scratch"
	corpusDir       = "playground/bookgen/out"
	corpusMainFile  = "book-corpus.txt"

	bookVisitFloor = 20
	bookMaxPly     = 16
	visitSteps     = "2,5,20"
	drawMovesFull  = 256

	wilsonZ = 1.96

	bucketIdentical = "identical"
	bucketSameBase  = "same-base"
	bucketCross     = "cross"
)

const (
	patArenaHeader = `^arena tc (\d+\+\d+) seats \[(.+?)\] games (\d+)$`
	patArenaResult = `^game (\d+): (.+) \(red\) vs (.+), winner (.+) over (\d+) moves$`
	patPonderLine  = `^g(\d+) ponder (\S+) m(\d+) (adopt|reject|miss)$`
	patCorpusF     = `^F (.+)$`
	patCorpusG     = `^G (\d+)$`
	patCorpusP     = `^P ([0-9a-fA-F]+) (\d+)$`
	patCorpusE     = `^E (\d+) (\d+)$`
	patMLine       = `^g(\d+) M(\d+), (Red|Blue), ([A-Pa-p][0-9]{1,2}),`
)
