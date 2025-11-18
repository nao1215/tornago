package tornago

import (
	"errors"
	"strings"
	"testing"
)

func TestTornagoError(t *testing.T) {
	t.Run("should create error with all fields populated", func(t *testing.T) {
		underlying := errors.New("underlying error")
		err := newError(ErrInvalidConfig, "TestOperation", "test message", underlying)

		if err == nil {
			t.Fatal("newError returned nil")
		}

		var te *TornagoError
		if !errors.As(err, &te) {
			t.Fatal("error is not TornagoError")
		}

		if te.Kind != ErrInvalidConfig {
			t.Errorf("Kind mismatch: want %v got %v", ErrInvalidConfig, te.Kind)
		}
		if te.Op != "TestOperation" {
			t.Errorf("Op mismatch: want %s got %s", "TestOperation", te.Op)
		}
		if !strings.Contains(te.Error(), "test message") {
			t.Errorf("Error message should contain 'test message': got %s", te.Error())
		}
	})

	t.Run("should unwrap to underlying error", func(t *testing.T) {
		underlying := errors.New("underlying error")
		err := newError(ErrInvalidConfig, "TestOperation", "test message", underlying)

		if !errors.Is(err, underlying) {
			t.Error("error should wrap underlying error")
		}
	})

	t.Run("should format error message correctly", func(t *testing.T) {
		err := newError(ErrInvalidConfig, "TestOp", "test message", nil)
		errStr := err.Error()

		if !strings.Contains(errStr, "TestOp") {
			t.Errorf("Error message should contain operation: %s", errStr)
		}
		if !strings.Contains(errStr, "test message") {
			t.Errorf("Error message should contain message: %s", errStr)
		}
	})
}

func TestErrorKinds(t *testing.T) {
	t.Run("should have distinct error kinds", func(t *testing.T) {
		kinds := []ErrorKind{
			ErrInvalidConfig,
			ErrTorBinaryNotFound,
			ErrTorLaunchFailed,
			ErrSocksDialFailed,
			ErrHTTPFailed,
			ErrControlRequestFail,
			ErrHiddenServiceFailed,
			ErrTimeout,
			ErrIO,
			ErrUnknown,
		}

		seen := make(map[ErrorKind]bool)
		for _, kind := range kinds {
			if seen[kind] {
				t.Errorf("Duplicate error kind: %v", kind)
			}
			seen[kind] = true
		}
	})

	t.Run("should differentiate between error kinds", func(t *testing.T) {
		err1 := newError(ErrInvalidConfig, "op", "msg", nil)
		err2 := newError(ErrTimeout, "op", "msg", nil)

		var te1, te2 *TornagoError
		if !errors.As(err1, &te1) || !errors.As(err2, &te2) {
			t.Fatal("errors are not TornagoError")
		}

		if te1.Kind == te2.Kind {
			t.Error("Different error kinds should not be equal")
		}
	})
}

func TestTornagoErrorIs(t *testing.T) {
	t.Run("should match error with same kind", func(t *testing.T) {
		err1 := newError(ErrInvalidConfig, "test", "test error", nil)
		err2 := &TornagoError{Kind: ErrInvalidConfig}

		if !errors.Is(err1, err2) {
			t.Error("expected Is to match errors with same Kind")
		}
	})

	t.Run("should not match different error kind", func(t *testing.T) {
		err1 := newError(ErrInvalidConfig, "test", "test error", nil)
		err2 := &TornagoError{Kind: ErrTorBinaryNotFound}

		if errors.Is(err1, err2) {
			t.Error("expected Is to not match errors with different Kind")
		}
	})

	t.Run("should not match non-TornagoError", func(t *testing.T) {
		err1 := newError(ErrInvalidConfig, "test", "test error", nil)
		err2 := errors.New("standard error")

		if errors.Is(err1, err2) {
			t.Error("expected Is to not match non-TornagoError")
		}
	})
}

func TestTornagoErrorUnwrap(t *testing.T) {
	t.Run("should unwrap to underlying error", func(t *testing.T) {
		underlying := errors.New("underlying error")
		err := newError(ErrInvalidConfig, "test", "test error", underlying)

		var te *TornagoError
		if !errors.As(err, &te) {
			t.Fatal("expected TornagoError")
		}

		unwrapped := te.Unwrap()
		if unwrapped == nil {
			t.Fatal("expected unwrapped error")
		}

		if unwrapped.Error() != "underlying error" {
			t.Errorf("expected 'underlying error', got %s", unwrapped.Error())
		}
	})

	t.Run("should return nil when no underlying error", func(t *testing.T) {
		err := newError(ErrInvalidConfig, "test", "test error", nil)

		var te *TornagoError
		if !errors.As(err, &te) {
			t.Fatal("expected TornagoError")
		}

		unwrapped := te.Unwrap()
		if unwrapped != nil {
			t.Errorf("expected nil unwrapped error, got %v", unwrapped)
		}
	})
}

func TestNewError(t *testing.T) {
	t.Run("should create error with all fields", func(t *testing.T) {
		underlying := errors.New("underlying")
		err := newError(ErrInvalidConfig, "testFunc", "test message", underlying)

		var te *TornagoError
		if !errors.As(err, &te) {
			t.Fatal("expected TornagoError")
		}

		if te.Kind != ErrInvalidConfig {
			t.Errorf("expected kind ErrInvalidConfig, got %v", te.Kind)
		}

		if te.Op != "testFunc" {
			t.Errorf("expected op 'testFunc', got %s", te.Op)
		}

		if te.Msg != "test message" {
			t.Errorf("expected message 'test message', got %s", te.Msg)
		}

		if te.Err == nil {
			t.Error("expected underlying error")
		}
	})

	t.Run("should create error without underlying error", func(t *testing.T) {
		err := newError(ErrTorBinaryNotFound, "testFunc", "test message", nil)

		var te *TornagoError
		if !errors.As(err, &te) {
			t.Fatal("expected TornagoError")
		}

		if te.Err != nil {
			t.Error("expected no underlying error")
		}
	})

	t.Run("should default to ErrUnknown when kind is empty", func(t *testing.T) {
		err := newError("", "testFunc", "test message", nil)

		var te *TornagoError
		if !errors.As(err, &te) {
			t.Fatal("expected TornagoError")
		}

		if te.Kind != ErrUnknown {
			t.Errorf("expected kind ErrUnknown, got %v", te.Kind)
		}
	})
}

func TestTornagoErrorNilHandling(t *testing.T) {
	t.Run("should handle nil error for Error() method", func(t *testing.T) {
		var err *TornagoError
		result := err.Error()
		if result != "" {
			t.Errorf("expected empty string for nil error, got %s", result)
		}
	})

	t.Run("should handle nil error for Unwrap() method", func(t *testing.T) {
		var err *TornagoError
		result := err.Unwrap()
		if result != nil {
			t.Errorf("expected nil for Unwrap on nil error, got %v", result)
		}
	})

	t.Run("should handle nil error for Is() method", func(t *testing.T) {
		var err *TornagoError
		target := &TornagoError{Kind: ErrTimeout}
		result := err.Is(target)
		if result {
			t.Error("expected false for Is on nil error")
		}
	})
}
