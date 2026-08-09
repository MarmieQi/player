package settings

import "testing"

func TestNormalizeLogQueryHourlyLimit(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{input: 0, want: 5},
		{input: -1, want: 5},
		{input: 1, want: 1},
		{input: 12, want: 12},
		{input: 61, want: 60},
	}
	for _, test := range tests {
		if got := NormalizeLogQueryHourlyLimit(test.input); got != test.want {
			t.Errorf("NormalizeLogQueryHourlyLimit(%d) = %d, want %d", test.input, got, test.want)
		}
	}
}
