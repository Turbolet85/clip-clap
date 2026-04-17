package lasterror

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

// resetForTest clears the package-level atomic.Value between tests.
// atomic.Value has no public Reset, but Store(errorHolder{nil}) achieves
// the equivalent — Get() returns nil for the zero-value holder.
func resetForTest(t *testing.T) {
	t.Helper()
	Set(nil)
}

// TestSet_StoresError is the happy path: write an error, read it back,
// message intact. Sanity-check for the atomic.Value round-trip.
func TestSet_StoresError(t *testing.T) {
	resetForTest(t)
	want := errors.New("test")
	Set(want)
	got := Get()
	if got == nil {
		t.Fatalf("Get() = nil, want non-nil error")
	}
	if got.Error() != "test" {
		t.Errorf("Get().Error() = %q, want %q", got.Error(), "test")
	}
}

// TestGet_ReturnsNilWhenEmpty checks the init-state and Set(nil) reset
// paths. Callers depend on nil-equivalent to mean "no error" for
// rendering the Last error menu slot as "<none>".
func TestGet_ReturnsNilWhenEmpty(t *testing.T) {
	resetForTest(t)
	if got := Get(); got != nil {
		t.Errorf("Get() after reset = %v, want nil", got)
	}
}

// TestSet_ConcurrentSafety exercises the goroutine-safety claim. Spawn
// N goroutines, each performs a Set with a distinct error, then a Get
// returns a non-nil value without panicking. The test tolerates any of
// the N errors winning — the contract is "no race, no panic", not
// "last writer wins deterministically".
func TestSet_ConcurrentSafety(t *testing.T) {
	resetForTest(t)
	const N = 10
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			Set(fmt.Errorf("err_%d", i))
		}()
	}
	wg.Wait()
	if got := Get(); got == nil {
		t.Errorf("Get() after concurrent writes = nil, want non-nil")
	}
}

// TestSet_DifferentConcreteTypes is the regression guard for the
// errorHolder wrapper — atomic.Value panics if the concrete type of
// the stored value changes between Stores. Subsystems publish errors
// of different concrete types (*errors.errorString, *os.PathError,
// *fmt.wrapError via fmt.Errorf, etc.); the wrapper must accept them
// all without panic.
func TestSet_DifferentConcreteTypes(t *testing.T) {
	resetForTest(t)
	Set(errors.New("errors.errorString"))
	Set(fmt.Errorf("fmt.wrapError: %w", errors.New("inner")))
	Set(&myCustomError{msg: "custom-struct-ptr"})
	if got := Get(); got == nil {
		t.Errorf("Get() after mixed-type writes = nil, want non-nil")
	}
}

type myCustomError struct{ msg string }

func (e *myCustomError) Error() string { return e.msg }
