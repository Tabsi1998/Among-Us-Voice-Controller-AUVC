package voice

import (
	"sync"
	"testing"
	"time"
)

// gateApplier holds its first change until released.
type gateApplier struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (g *gateApplier) Apply(string, Change) error {
	g.once.Do(func() { close(g.entered) })
	<-g.release
	return nil
}

func TestExclusiveWorkWaitsForAReconciliationOfTheSameGuild(t *testing.T) {
	gate := &gateApplier{entered: make(chan struct{}), release: make(chan struct{})}
	reconciler := NewReconciler(gate)

	reconciled := make(chan struct{})
	go func() {
		_ = reconciler.Reconcile("guild",
			map[string]Observed{"member": {ChannelID: "main"}},
			map[string]DesiredVoiceState{"member": {TargetChannelID: "ghost"}})
		close(reconciled)
	}()
	<-gate.entered

	ran := make(chan struct{})
	go reconciler.Exclusive("guild", func() { close(ran) })

	select {
	case <-ran:
		t.Fatal("exclusive work ran in the middle of a reconciliation")
	case <-time.After(50 * time.Millisecond):
	}

	close(gate.release)
	<-reconciled
	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("exclusive work never ran")
	}
}

func TestExclusiveWorkForAnotherGuildDoesNotWait(t *testing.T) {
	gate := &gateApplier{entered: make(chan struct{}), release: make(chan struct{})}
	reconciler := NewReconciler(gate)
	defer close(gate.release)

	go func() {
		_ = reconciler.Reconcile("busy",
			map[string]Observed{"member": {ChannelID: "main"}},
			map[string]DesiredVoiceState{"member": {TargetChannelID: "ghost"}})
	}()
	<-gate.entered

	ran := make(chan struct{})
	go reconciler.Exclusive("other", func() { close(ran) })
	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("another guild waited for a reconciliation it has nothing to do with")
	}
}
