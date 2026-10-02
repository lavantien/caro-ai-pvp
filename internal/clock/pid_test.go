package clock

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestPIDPureProportional(t *testing.T) {
	g := config.PIDGains{Kp: 0.02, Ki: 0, Kd: 0}
	p := newPID(g)
	if got, want := p.step(100), g.Kp*100; got != want {
		t.Errorf("first step = %v, want %v", got, want)
	}
	if got, want := p.step(50), g.Kp*50; got != want {
		t.Errorf("second step = %v, want %v (no integral or derivative action)", got, want)
	}
	if got, want := p.step(-75), g.Kp*-75; got != want {
		t.Errorf("third step = %v, want %v", got, want)
	}
}

func TestPIDIntegralAccumulates(t *testing.T) {
	g := config.PIDGains{Kp: 0, Ki: 0.01, Kd: 0}
	p := newPID(g)
	if got, want := p.step(10), g.Ki*10; got != want {
		t.Errorf("after one step = %v, want %v", got, want)
	}
	if got, want := p.step(10), g.Ki*(10+10); got != want {
		t.Errorf("after two steps = %v, want %v", got, want)
	}
	if got, want := p.step(10), g.Ki*((10+10)+10); got != want {
		t.Errorf("after three steps = %v, want %v", got, want)
	}
}

func TestPIDIntegralWindupClampsAccumulator(t *testing.T) {
	// The accumulated error itself is clamped at plus or minus
	// ClockPIDIntegClampMs, so the Ki-scaled term may exceed the bound when
	// Ki > 1: that discriminates accumulator clamping from term clamping.
	g := config.PIDGains{Kp: 0, Ki: 2, Kd: 0}
	p := newPID(g)
	if got, want := p.step(600), g.Ki*500; got != want {
		t.Errorf("oversized step = %v, want %v (accumulator clamped at %v)", got, want, config.ClockPIDIntegClampMs)
	}
	if got, want := p.step(600), g.Ki*500; got != want {
		t.Errorf("saturated step = %v, want %v", got, want)
	}
	if got, want := p.step(-600), g.Ki*(500-600); got != want {
		t.Errorf("post-saturation step = %v, want %v (clamped accumulator bleeds off)", got, want)
	}
	n := newPID(g)
	if got, want := n.step(-2400), g.Ki*-500; got != want {
		t.Errorf("negative saturation = %v, want %v", got, want)
	}
}

func TestPIDDerivative(t *testing.T) {
	g := config.PIDGains{Kp: 0, Ki: 0, Kd: 0.005}
	p := newPID(g)
	if got := p.step(42); got != 0 {
		t.Errorf("first step derivative = %v, want 0 (prevErr primed)", got)
	}
	prev := 42.0
	for _, err := range []float64{50, 35, 35} {
		if got, want := p.step(err), g.Kd*(err-prev); got != want {
			t.Errorf("derivative after step(%v) = %v, want %v", err, got, want)
		}
		prev = err
	}
}

func TestPIDFullStep(t *testing.T) {
	g := config.PIDGains{Kp: 0.02, Ki: 0.01, Kd: 0.005}
	p := newPID(g)
	integ, prev := 0.0, 0.0
	first := true
	for _, err := range []float64{100, -100, -100} {
		want := g.Kp*err + g.Ki*(integ+err)
		if !first {
			want += g.Kd * (err - prev)
		}
		integ += err
		prev = err
		first = false
		if got := p.step(err); got != want {
			t.Errorf("step(%v) = %v, want %v", err, got, want)
		}
	}
}

func TestPIDResetClearsState(t *testing.T) {
	g := config.PIDGains{Kp: 0.02, Ki: 0.01, Kd: 0.005}
	p := newPID(g)
	p.step(600)
	p.step(600)
	p.reset()
	// After reset the accumulator is empty and prevErr unprimed: identical
	// to a fresh controller fed the same error.
	fresh := newPID(g)
	if got, want := p.step(42), fresh.step(42); got != want {
		t.Errorf("post-reset step = %v, want fresh step %v", got, want)
	}
	if got, want := p.step(42), fresh.step(42); got != want {
		t.Errorf("post-reset drift = %v, want fresh drift %v", got, want)
	}
}
