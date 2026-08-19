package algo_test

import (
	"cmp"
	"slices"
	"testing"

	"github.com/azuiktech/kleisli-go/algo"
)

// Example: s=[7 8 3 4 5 6], pivot=2
//   first  = s[pivot:] = [3 4 5 6]  (older/lower, Segments first)
//   second = s[:pivot] = [7 8]      (newer/higher, Segments second)
//   logical order: [3 4 5 6 7 8]

var (
	first  = []int{3, 4, 5, 6}
	second = []int{7, 8}
)

func cmpInt(a, b int) int { return cmp.Compare(a, b) }

func TestFindRotated_InFirst(t *testing.T) {
	if n := algo.FindRotated(first, second, 4, cmpInt); n != 1 {
		t.Fatalf("want 1, got %d", n)
	}
}

func TestFindRotated_InSecond(t *testing.T) {
	if n := algo.FindRotated(first, second, 7, cmpInt); n != 4 {
		t.Fatalf("want 4, got %d", n)
	}
}

func TestFindRotated_NotFound(t *testing.T) {
	want := len(first) + len(second)
	if n := algo.FindRotated(first, second, 9, cmpInt); n != want {
		t.Fatalf("want %d (not found), got %d", want, n)
	}
}

func TestAfter_InFirst_Bitonic(t *testing.T) {
	// after 4 (n=1 in first): [5 6], [7 8]
	a, b := algo.After(first, second, 1)
	if !slices.Equal(a, []int{5, 6}) || !slices.Equal(b, []int{7, 8}) {
		t.Fatalf("want [5 6],[7 8] got %v,%v", a, b)
	}
}

func TestAfter_InSecond_Monotonic(t *testing.T) {
	// after 7 (n=4 in second, offset 0): [8], nil
	a, b := algo.After(first, second, 4)
	if !slices.Equal(a, []int{8}) || len(b) != 0 {
		t.Fatalf("want [8],nil got %v,%v", a, b)
	}
}

func TestAfter_Last(t *testing.T) {
	// after 8 (n=5, last): nothing
	a, b := algo.After(first, second, 5)
	if len(a) != 0 || len(b) != 0 {
		t.Fatalf("want empty,nil got %v,%v", a, b)
	}
}

func TestBefore_InFirst_Monotonic(t *testing.T) {
	// before 4 (n=1): [3], nil
	a, b := algo.Before(first, second, 1)
	if !slices.Equal(a, []int{3}) || len(b) != 0 {
		t.Fatalf("want [3],nil got %v,%v", a, b)
	}
}

func TestBefore_InSecond_Bitonic(t *testing.T) {
	// before 8 (n=5, second offset 1): [3 4 5 6], [7]
	a, b := algo.Before(first, second, 5)
	if !slices.Equal(a, []int{3, 4, 5, 6}) || !slices.Equal(b, []int{7}) {
		t.Fatalf("want [3 4 5 6],[7] got %v,%v", a, b)
	}
}

func TestBefore_First_Element(t *testing.T) {
	// before 3 (n=0, very first): nothing
	a, b := algo.Before(first, second, 0)
	if len(a) != 0 || len(b) != 0 {
		t.Fatalf("want nil,nil got %v,%v", a, b)
	}
}

func TestBefore_FirstOfSecond(t *testing.T) {
	// before 7 (n=4, first of second): all of first, nil
	a, b := algo.Before(first, second, 4)
	if !slices.Equal(a, []int{3, 4, 5, 6}) || len(b) != 0 {
		t.Fatalf("want [3 4 5 6],nil got %v,%v", a, b)
	}
}

func TestFindRotatedWithPivot(t *testing.T) {
	s := []int{7, 8, 3, 4, 5, 6}
	pivot := 2
	cases := []struct{ val, want int }{
		{3, 2}, {4, 3}, {5, 4}, {6, 5}, {7, 0}, {8, 1},
	}
	for _, tc := range cases {
		if got := algo.FindRotatedWithPivot(s, pivot, tc.val, cmpInt); got != tc.want {
			t.Errorf("Find(%d): want %d got %d", tc.val, tc.want, got)
		}
	}
}

func TestFindRotatedWithPivot_NotFound(t *testing.T) {
	s := []int{7, 8, 3, 4, 5, 6}
	if got := algo.FindRotatedWithPivot(s, 2, 9, cmpInt); got != len(s) {
		t.Fatalf("want %d (not found), got %d", len(s), got)
	}
}

func TestAfterWithPivot(t *testing.T) {
	s := []int{7, 8, 3, 4, 5, 6}
	// pos=3 (val=4): after = [5 6],[7 8]
	a, b := algo.AfterWithPivot(s, 3, 2)
	if !slices.Equal(a, []int{5, 6}) || !slices.Equal(b, []int{7, 8}) {
		t.Fatalf("want [5 6],[7 8] got %v,%v", a, b)
	}
	// pos=0 (val=7): after = [8],nil
	a, b = algo.AfterWithPivot(s, 0, 2)
	if !slices.Equal(a, []int{8}) || len(b) != 0 {
		t.Fatalf("want [8],nil got %v,%v", a, b)
	}
}

func TestBeforeWithPivot(t *testing.T) {
	s := []int{7, 8, 3, 4, 5, 6}
	// pos=3 (val=4): before = [3],nil
	a, b := algo.BeforeWithPivot(s, 3, 2)
	if !slices.Equal(a, []int{3}) || len(b) != 0 {
		t.Fatalf("want [3],nil got %v,%v", a, b)
	}
	// pos=0 (val=7): before = [3 4 5 6],nil
	a, b = algo.BeforeWithPivot(s, 0, 2)
	if !slices.Equal(a, []int{3, 4, 5, 6}) || len(b) != 0 {
		t.Fatalf("want [3 4 5 6],nil got %v,%v", a, b)
	}
}
