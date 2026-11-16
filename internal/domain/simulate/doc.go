// Package simulate holds the deterministic what-if engines: investment/PAC
// growth and hypothetical cash-flow scenarios. It never touches storage.
//
// It is the only domain package allowed to use float64: growth factors are
// computed in floating point and rounded to int64 minor units at the API
// boundary, so no float leaks out.
package simulate
