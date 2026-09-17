package handlers

import (
	"math"
	"testing"
)

func TestCalculateCommentRatio(t *testing.T) {
	tests := []struct {
		name     string
		comments []Comment
		answer   int
		want     *float64
	}{
		{name: "no comments", answer: 1, want: nil},
		{name: "invalid answer", comments: []Comment{{Choice: 1}}, answer: 0, want: nil},
		{name: "ignores invalid choices", comments: []Comment{{Choice: 0}, {Choice: 3}}, answer: 1, want: nil},
		{name: "all matching", comments: []Comment{{Choice: 2}, {Choice: 2}}, answer: 2, want: float64Pointer(100)},
		{name: "none matching", comments: []Comment{{Choice: 1}, {Choice: 1}}, answer: 2, want: float64Pointer(0)},
		{name: "uses only choices one and two", comments: []Comment{{Choice: 1}, {Choice: 2}, {Choice: 1}, {Choice: 0}}, answer: 1, want: float64Pointer(200.0 / 3.0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCommentRatio(tt.comments, tt.answer)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("calculateCommentRatio() = %v, want nil", *got)
				}
				return
			}
			if got == nil || math.Abs(*got-*tt.want) > 1e-9 {
				t.Fatalf("calculateCommentRatio() = %v, want %v", got, *tt.want)
			}
		})
	}
}

func float64Pointer(value float64) *float64 {
	return &value
}
