package clock

import (
	"math"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// pid is the parallel-form controller steering one side's budget against its
// planned drain trajectory. Errors and output are milliseconds.
type pid struct {
	gains   config.PIDGains
	integ   float64
	prevErr float64
	primed  bool
}

func newPID(gains config.PIDGains) pid {
	return pid{gains: gains}
}

// step advances the controller by one move's error sample. The accumulated
// integral itself is clamped, so saturation bleeds off only through new
// error instead of persisting in the scaled term, and prevErr is primed on
// the first step so the derivative action starts at zero.
func (p *pid) step(err float64) float64 {
	p.integ = clamp(p.integ+err, -config.ClockPIDIntegClampMs, config.ClockPIDIntegClampMs)
	deriv := 0.0
	if p.primed {
		deriv = p.gains.Kd * (err - p.prevErr)
	}
	p.prevErr = err
	p.primed = true
	return p.gains.Kp*err + p.gains.Ki*p.integ + deriv
}

func (p *pid) reset() {
	p.integ = 0
	p.prevErr = 0
	p.primed = false
}

func clamp(v, lo, hi float64) float64 {
	return math.Min(math.Max(v, lo), hi)
}
