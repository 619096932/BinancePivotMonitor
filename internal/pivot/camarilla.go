package pivot

import "errors"

type Levels struct {
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
	PP    float64 `json:"pp"`
	R1    float64 `json:"r1"`
	R2    float64 `json:"r2"`
	R3    float64 `json:"r3"`
	R4    float64 `json:"r4"`
	R5    float64 `json:"r5"`
	S1    float64 `json:"s1"`
	S2    float64 `json:"s2"`
	S3    float64 `json:"s3"`
	S4    float64 `json:"s4"`
	S5    float64 `json:"s5"`
}

func Calculate(high, low, close float64) (Levels, error) {
	if high <= 0 || low <= 0 {
		return Levels{}, errors.New("invalid high/low")
	}
	if high < low {
		return Levels{}, errors.New("high < low")
	}

	rng := high - low
	factor := 1.1

	r1 := close + rng*factor/12.0
	r2 := close + rng*factor/6.0
	r3 := close + rng*factor/4.0
	r4 := close + rng*factor/2.0

	r5 := (high / low) * close
	s5 := close - (r5 - close)

	s1 := close - rng*factor/12.0
	s2 := close - rng*factor/6.0
	s3 := close - rng*factor/4.0
	s4 := close - rng*factor/2.0

	return Levels{
		High:  high,
		Low:   low,
		Close: close,
		PP:    (high + low + close) / 3.0,
		R1:    r1,
		R2:    r2,
		R3:    r3,
		R4:    r4,
		R5:    r5,
		S1:    s1,
		S2:    s2,
		S3:    s3,
		S4:    s4,
		S5:    s5,
	}, nil
}
