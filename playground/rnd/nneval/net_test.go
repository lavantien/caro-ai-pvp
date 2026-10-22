package main

import (
	"math"
	"slices"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func referenceForward(n *net, input []float32) float32 {
	src := slices.Clone(input)
	for l := range n.layers {
		ly := &n.layers[l]
		dst := make([]float32, ly.out)
		for j := range ly.out {
			sum := ly.b[j]
			for i := range ly.in {
				sum += src[i] * ly.w[j*ly.in+i]
			}
			if l < len(n.layers)-1 && sum < 0 {
				sum = 0
			}
			dst[j] = sum
		}
		src = dst
	}
	return src[0]
}

func TestForwardMatchesReference(t *testing.T) {
	n := newNet([]int{8, 4, 2, 1}, weightSeedA, weightSeedB)
	inputs := batchInputs(16, 8, inputSeedA, inputSeedB)
	out := make([]float32, len(inputs))
	n.forwardBatch(inputs, out, 1)
	for s := range inputs {
		want := referenceForward(n, inputs[s])
		if out[s] != want {
			t.Fatalf("sample %d: got %f want %f", s, out[s], want)
		}
	}
}

func TestForwardDeterministic(t *testing.T) {
	a := newNet([]int{32, 16, 8, 1}, weightSeedA, weightSeedB)
	b := newNet([]int{32, 16, 8, 1}, weightSeedA, weightSeedB)
	inputs := batchInputs(4, 32, inputSeedA, inputSeedB)
	oa := make([]float32, len(inputs))
	ob := make([]float32, len(inputs))
	a.forwardBatch(inputs, oa, 1)
	b.forwardBatch(inputs, ob, 1)
	for s := range inputs {
		if oa[s] != ob[s] {
			t.Fatalf("same seed nets disagree at %d: %f vs %f", s, oa[s], ob[s])
		}
	}
}

func TestParallelMatchesSerial(t *testing.T) {
	n := newNet([]int{64, 32, 16, 1}, weightSeedA, weightSeedB)
	inputs := batchInputs(64, 64, inputSeedA, inputSeedB)
	serial := make([]float32, len(inputs))
	par := make([]float32, len(inputs))
	n.forwardBatch(inputs, serial, 1)
	n.forwardBatch(inputs, par, 4)
	for s := range inputs {
		if serial[s] != par[s] {
			t.Fatalf("sample %d: serial %f parallel %f", s, serial[s], par[s])
		}
	}
}

func TestBatchMatchesSingle(t *testing.T) {
	n := newNet(netDims[netNarrow], weightSeedA, weightSeedB)
	inputs := batchInputs(7, netDims[netNarrow][0], inputSeedA, inputSeedB)
	batched := make([]float32, len(inputs))
	n.forwardBatch(inputs, batched, 1)
	scratch := n.makeScratch()
	for s := range inputs {
		var single float32
		n.forwardSample(inputs[s], scratch, &single)
		if batched[s] != single {
			t.Fatalf("sample %d: batch %f single %f", s, batched[s], single)
		}
	}
}

func TestReluZeroesHidden(t *testing.T) {
	n := newNet([]int{4, 3, 2, 1}, weightSeedA, weightSeedB)
	for l := range n.layers[:len(n.layers)-1] {
		for j := range n.layers[l].b {
			n.layers[l].b[j] = -1e9
		}
		for i := range n.layers[l].w {
			n.layers[l].w[i] = 0
		}
	}
	n.layers[2].b[0] = 0.5
	for i := range n.layers[2].w {
		n.layers[2].w[i] = 0
	}
	inputs := batchInputs(3, 4, inputSeedA, inputSeedB)
	out := make([]float32, len(inputs))
	n.forwardBatch(inputs, out, 1)
	for s := range out {
		if out[s] != 0.5 {
			t.Fatalf("sample %d: got %f want 0.5 from the only live bias", s, out[s])
		}
	}
}

func TestBatchInputsDeterministic(t *testing.T) {
	a := batchInputs(5, 16, inputSeedA, inputSeedB)
	b := batchInputs(5, 16, inputSeedA, inputSeedB)
	for s := range a {
		if !slices.Equal(a[s], b[s]) {
			t.Fatalf("input %d differs between calls with the same seed", s)
		}
	}
}

func TestWeightsBounded(t *testing.T) {
	for slot := range netCount {
		n := newNet(netDims[slot], weightSeedA, weightSeedB)
		for l := range n.layers {
			ly := &n.layers[l]
			limit := float32(4 / math.Sqrt(float64(ly.in)))
			for _, w := range ly.w {
				if w > limit || w < -limit {
					t.Fatalf("slot %d layer %d weight %f outside init bound %f", slot, l, w, limit)
				}
			}
		}
	}
}

func TestMidgameBoardFeedsEngine(t *testing.T) {
	b, err := midgameBoard()
	if err != nil {
		t.Fatal(err)
	}
	if b.IsFull() {
		t.Fatal("midgame board is full")
	}
	mv, stats := probeBestMove(b)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("engine returned illegal move %d", mv)
	}
	if stats.Nodes == 0 {
		t.Fatal("engine searched zero nodes")
	}
}
