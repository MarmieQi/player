package httpapi

import (
	"math"
	"net/http/httptest"
	"testing"
)

func TestQueryFloatRejectsNonFiniteValues(t *testing.T) {
	for _, raw := range []string{"NaN", "Inf", "-Inf"} {
		request := httptest.NewRequest("GET", "/?x="+raw, nil)
		response := httptest.NewRecorder()
		if _, ok := queryFloat(response, request, "x"); ok {
			t.Fatalf("queryFloat accepted %q", raw)
		}
		if response.Code != 400 {
			t.Fatalf("queryFloat(%q) status = %d, want 400", raw, response.Code)
		}
	}
}

func TestQueryFloatAcceptsFiniteValue(t *testing.T) {
	request := httptest.NewRequest("GET", "/?x=-191.5", nil)
	response := httptest.NewRecorder()
	value, ok := queryFloat(response, request, "x")
	if !ok {
		t.Fatal("queryFloat rejected finite value")
	}
	if math.Abs(value-(-191.5)) > 1e-9 {
		t.Fatalf("queryFloat value = %v, want -191.5", value)
	}
}

func TestQueryOptionalFloatUsesFallback(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	response := httptest.NewRecorder()
	value, ok := queryOptionalFloat(response, request, "radius", 0)
	if !ok || value != 0 {
		t.Fatalf("queryOptionalFloat() = (%v, %v), want (0, true)", value, ok)
	}
}
