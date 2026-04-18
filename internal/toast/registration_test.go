package toast

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// mockRegistry records OpenKey / CreateKey / SetStringValue call counts so
// the test can distinguish a registry-checked-each-call implementation (correct)
// from a memory-cached implementation (wrong, breaks cross-process idempotency).
type mockRegistry struct {
	openCalls     int
	createCalls   int
	setValueCalls int

	// keyExists controls what OpenKey returns. First call can return
	// ErrNotExist to exercise the "first-time create" path; second call
	// can return nil (key found) to exercise the idempotent short-circuit.
	keyExistsAt []bool // index by openCalls-1
}

func (m *mockRegistry) openFn(k registry.Key, path string, access uint32) (registry.Key, error) {
	idx := m.openCalls
	m.openCalls++
	if idx < len(m.keyExistsAt) && m.keyExistsAt[idx] {
		return registry.Key(0), nil
	}
	return registry.Key(0), registry.ErrNotExist
}

func (m *mockRegistry) createFn(k registry.Key, path string, access uint32) (registry.Key, bool, error) {
	m.createCalls++
	return registry.Key(0), false, nil
}

func (m *mockRegistry) setFn(k registry.Key, name, value string) error {
	m.setValueCalls++
	return nil
}

func (m *mockRegistry) closeFn(k registry.Key) error { return nil }

func TestRegisterAppUserModelID_Idempotent(t *testing.T) {
	mock := &mockRegistry{
		// First call: key does NOT exist → create + set. Second call: key
		// EXISTS → short-circuit, no create, no set.
		keyExistsAt: []bool{false, true},
	}
	SetRegistryFunctionsForTesting(mock.openFn, mock.createFn, mock.setFn, mock.closeFn)
	t.Cleanup(ResetRegistryFunctions)

	// First call: expect OpenKey invoked, then CreateKey + SetStringValue.
	if err := RegisterAppUserModelID(DefaultAppID); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if mock.openCalls != 1 {
		t.Errorf("after 1st call: openCalls = %d, want 1", mock.openCalls)
	}
	if mock.createCalls != 1 {
		t.Errorf("after 1st call: createCalls = %d, want 1", mock.createCalls)
	}
	if mock.setValueCalls != 1 {
		t.Errorf("after 1st call: setValueCalls = %d, want 1", mock.setValueCalls)
	}

	// Second call: expect OpenKey invoked AGAIN (registry re-check, not
	// memory cache), but CreateKey + SetStringValue NOT invoked.
	if err := RegisterAppUserModelID(DefaultAppID); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if mock.openCalls != 2 {
		t.Errorf("after 2nd call: openCalls = %d, want 2 (registry re-check, not memory cache)", mock.openCalls)
	}
	if mock.createCalls != 1 {
		t.Errorf("after 2nd call: createCalls = %d, want 1 (idempotent: no new write)", mock.createCalls)
	}
	if mock.setValueCalls != 1 {
		t.Errorf("after 2nd call: setValueCalls = %d, want 1 (idempotent: no new write)", mock.setValueCalls)
	}
}

func TestRegisterAppUserModelID_CreateFailurePropagates(t *testing.T) {
	expected := errors.New("simulated create failure")
	SetRegistryFunctionsForTesting(
		func(k registry.Key, path string, access uint32) (registry.Key, error) {
			return registry.Key(0), registry.ErrNotExist
		},
		func(k registry.Key, path string, access uint32) (registry.Key, bool, error) {
			return registry.Key(0), false, expected
		},
		nil, nil,
	)
	t.Cleanup(ResetRegistryFunctions)

	err := RegisterAppUserModelID(DefaultAppID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expected) {
		t.Errorf("err = %v, want wrap of %v", err, expected)
	}
}
