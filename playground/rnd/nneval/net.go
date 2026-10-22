package main

import (
	"math"
	"math/rand/v2"
	"sync"
)

type layer struct {
	in, out int
	w       []float32
	b       []float32
}

type net struct {
	dims   []int
	layers []layer
}

func newNet(dims []int, seedA, seedB uint64) *net {
	n := &net{dims: dims}
	rng := rand.New(rand.NewPCG(seedA, seedB))
	for l := range len(dims) - 1 {
		in, out := dims[l], dims[l+1]
		scale := float32(1 / math.Sqrt(float64(in)))
		w := make([]float32, in*out)
		for i := range w {
			w[i] = float32(rng.Float64())*2*scale - scale
		}
		b := make([]float32, out)
		for i := range b {
			b[i] = float32(rng.Float64())*2*scale - scale
		}
		n.layers = append(n.layers, layer{in: in, out: out, w: w, b: b})
	}
	return n
}

func (n *net) makeScratch() [][]float32 {
	s := make([][]float32, len(n.layers))
	for l := range n.layers {
		s[l] = make([]float32, n.layers[l].out)
	}
	return s
}

func (n *net) forwardSample(input []float32, scratch [][]float32, out *float32) {
	src := input
	for l := range n.layers {
		ly := &n.layers[l]
		dst := scratch[l]
		for j := range ly.out {
			sum := ly.b[j]
			row := ly.w[j*ly.in : (j+1)*ly.in]
			for i, x := range src {
				sum += x * row[i]
			}
			if l < len(n.layers)-1 && sum < 0 {
				sum = 0
			}
			dst[j] = sum
		}
		src = dst
	}
	*out = scratch[len(scratch)-1][0]
}

func (n *net) forwardBatch(inputs [][]float32, outputs []float32, threads int) {
	if threads <= 1 {
		scratch := n.makeScratch()
		for s := range inputs {
			n.forwardSample(inputs[s], scratch, &outputs[s])
		}
		return
	}
	var wg sync.WaitGroup
	wg.Add(threads)
	chunk := (len(inputs) + threads - 1) / threads
	for t := range threads {
		lo := t * chunk
		hi := min(lo+chunk, len(inputs))
		go func() {
			defer wg.Done()
			scratch := n.makeScratch()
			for s := lo; s < hi; s++ {
				n.forwardSample(inputs[s], scratch, &outputs[s])
			}
		}()
	}
	wg.Wait()
}

func batchInputs(batch, inLen int, seedA, seedB uint64) [][]float32 {
	rng := rand.New(rand.NewPCG(seedA, seedB))
	inputs := make([][]float32, batch)
	for s := range inputs {
		v := make([]float32, inLen)
		for i := range v {
			v[i] = float32(rng.Float64())*2 - 1
		}
		inputs[s] = v
	}
	return inputs
}

func medianF(v []float64) float64 {
	for i := range v {
		for j := i + 1; j < len(v); j++ {
			if v[j] < v[i] {
				v[i], v[j] = v[j], v[i]
			}
		}
	}
	return v[len(v)/2]
}
