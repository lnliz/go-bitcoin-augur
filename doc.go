// Package augur estimates Bitcoin transaction fees from historical mempool
// snapshots using a Poisson block-arrival model and observed transaction inflows.
// Fees are satoshis, transaction weights are weight units, and estimated rates
// are satoshis per virtual byte. The package performs no I/O; callers collect
// snapshots, persist history, and decide how old an estimate may be before use.
package augur
