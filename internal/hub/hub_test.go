package hub

import (
	"sync"
	"testing"
	"time"

	"github.com/IliaPopov28/websocket-chat/internal/domain"
)

type stubClient struct {
	nickname   string
	closeCh    chan struct{}
	mu         sync.Mutex
	closeCount int
}

func newStubClient(nickname string) *stubClient {
	return &stubClient{
		nickname: nickname,
		closeCh:  make(chan struct{}, 1),
	}
}

func (c *stubClient) Nickname() string {
	return c.nickname
}

func (c *stubClient) Send(*domain.Message) {}

func (c *stubClient) Close() {
	c.mu.Lock()
	c.closeCount++
	c.mu.Unlock()

	select {
	case c.closeCh <- struct{}{}:
	default:
	}
}

func (c *stubClient) CloseCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeCount
}

func TestHubShutdownClosesRegisteredClients(t *testing.T) {
	t.Parallel()

	h := NewHub()
	runDone := make(chan struct{})
	go func() {
		h.Run()
		close(runDone)
	}()

	alice := newStubClient("alice")
	bob := newStubClient("bob")

	if !h.RegisterWithResult(alice) {
		t.Fatal("expected alice to register successfully")
	}
	if !h.RegisterWithResult(bob) {
		t.Fatal("expected bob to register successfully")
	}

	h.Shutdown()

	waitForSignal(t, runDone, "hub run loop should stop after shutdown")
	waitForSignal(t, alice.closeCh, "alice should be disconnected during shutdown")
	waitForSignal(t, bob.closeCh, "bob should be disconnected during shutdown")

	if alice.CloseCount() != 1 {
		t.Fatalf("expected alice to be closed once, got %d", alice.CloseCount())
	}
	if bob.CloseCount() != 1 {
		t.Fatalf("expected bob to be closed once, got %d", bob.CloseCount())
	}
}

func TestHubRegisterWithResultRejectsDuplicateNickname(t *testing.T) {
	t.Parallel()

	h := NewHub()
	runDone := make(chan struct{})
	go func() {
		h.Run()
		close(runDone)
	}()
	defer func() {
		h.Shutdown()
		waitForSignal(t, runDone, "hub should stop at the end of duplicate nickname test")
	}()

	firstTab := newStubClient("alice")
	secondTab := newStubClient("alice")

	if !h.RegisterWithResult(firstTab) {
		t.Fatal("expected first tab to register successfully")
	}
	if h.RegisterWithResult(secondTab) {
		t.Fatal("expected second tab with duplicate nickname to be rejected")
	}

	if secondTab.CloseCount() != 0 {
		t.Fatalf("duplicate registration must not close unrelated client state, got %d closes", secondTab.CloseCount())
	}
}

func waitForSignal(t *testing.T, ch <-chan struct{}, message string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal(message)
	}
}
