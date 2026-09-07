package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

type mempoolSourceFunc func(context.Context) (mempoolObservation, error)

func (f mempoolSourceFunc) GetMempool(ctx context.Context) (mempoolObservation, error) {
	return f(ctx)
}

type testStore struct {
	snapshots []augur.MempoolSnapshot
	saveErr   error
}

func (s *testStore) SaveSnapshot(snapshot augur.MempoolSnapshot) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.snapshots = append(s.snapshots, snapshot)
	return nil
}
func (s *testStore) GetSnapshots(start, end time.Time) ([]augur.MempoolSnapshot, error) {
	return s.snapshots, nil
}

func TestCollectorStopCancelsRPCAndIsIdempotent(t *testing.T) {
	entered := make(chan struct{}, 1)
	source := mempoolSourceFunc(func(ctx context.Context) (mempoolObservation, error) {
		entered <- struct{}{}
		<-ctx.Done()
		return mempoolObservation{}, ctx.Err()
	})
	c := NewMempoolCollector(source, nil, nil)
	c.Stop()
	for range 2 {
		c.Start()
		c.Start()
		<-entered
		done := make(chan struct{})
		go func() { c.Stop(); c.Stop(); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("collector shutdown did not cancel RPC")
		}
	}
}

func TestCollectorConcurrentStartStop(t *testing.T) {
	c := NewMempoolCollector(mempoolSourceFunc(func(ctx context.Context) (mempoolObservation, error) {
		<-ctx.Done()
		return mempoolObservation{}, ctx.Err()
	}), nil, nil)
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { c.Start(); c.Stop() })
	}
	wg.Wait()
	c.Stop()
}

func TestCollectorFreshness(t *testing.T) {
	c := &MempoolCollector{}
	if c.GetLatestFeeEstimate() != nil {
		t.Fatal("unexpected estimate")
	}
	for _, age := range []time.Duration{time.Second, 3 * time.Minute, -time.Minute} {
		c.latest.Store(&collectedEstimates{estimate: augur.FeeEstimate{Timestamp: time.Now().Add(-age)}})
		if got := c.GetLatestFeeEstimate() != nil; got != (age == time.Second) {
			t.Errorf("age %s: available=%v", age, got)
		}
	}
}

func TestCollectorPublishesOnlyAfterSuccessfulPersistence(t *testing.T) {
	estimator, err := augur.NewFeeEstimator()
	if err != nil {
		t.Fatal(err)
	}
	store := &testStore{saveErr: errors.New("disk full")}
	c := NewMempoolCollector(mempoolSourceFunc(func(context.Context) (mempoolObservation, error) {
		return mempoolObservation{BlockHeight: 800000, BlockHash: "tip", Transactions: []augur.MempoolTransaction{{Weight: 400, Fee: 100}}}, nil
	}), store, estimator)
	if err := c.updateFeeEstimates(context.Background()); err == nil {
		t.Fatal("ignored persistence failure")
	}
	if c.GetLatestFeeEstimate() != nil {
		t.Fatal("published estimate without saved snapshot")
	}
	store.saveErr = nil
	if err := c.updateFeeEstimates(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.snapshots) == 0 || store.snapshots[0].BlockHash != "tip" {
		t.Fatal("collector lost block hash")
	}
	if c.GetLatestFeeEstimate() == nil {
		t.Fatal("did not publish estimate")
	}
}

func TestCollectorHistoricalFreshness(t *testing.T) {
	estimator, err := augur.NewFeeEstimator()
	if err != nil {
		t.Fatal(err)
	}
	target := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	store := &testStore{}
	c := NewMempoolCollector(nil, store, estimator)
	for _, age := range []time.Duration{time.Minute, 3 * time.Minute} {
		store.snapshots = []augur.MempoolSnapshot{augur.NewEmptyMempoolSnapshot(1, target.Add(-age))}
		estimate, err := c.GetFeeEstimateForTimestamp(target.Unix())
		if err != nil {
			t.Fatal(err)
		}
		if got := estimate != nil; got != (age == time.Minute) {
			t.Errorf("age %s: available=%v", age, got)
		}
	}
	store.snapshots = nil
	if estimate, err := c.GetLatestFeeEstimateForBlockTarget(3); err != nil || estimate != nil {
		t.Fatalf("empty store: estimate=%v err=%v", estimate, err)
	}
}
