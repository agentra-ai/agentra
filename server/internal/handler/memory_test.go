package handler

import "testing"

func TestValidMemoryType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		memoryType string
		want       bool
	}{
		{memoryType: "learning", want: true},
		{memoryType: "task_result", want: true},
		{memoryType: "context", want: true},
		{memoryType: "pattern", want: true},
		{memoryType: "", want: false},
		{memoryType: "unsupported", want: false},
	}

	for _, test := range tests {
		t.Run(test.memoryType, func(t *testing.T) {
			t.Parallel()
			if got := validMemoryType(test.memoryType); got != test.want {
				t.Fatalf("validMemoryType(%q) = %v, want %v", test.memoryType, got, test.want)
			}
		})
	}
}
