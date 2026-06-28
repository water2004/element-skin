package permission_test

import (
	"testing"

	"element-skin/backend/internal/permission"
)

func TestBitSetBooleanOperationsExactly(t *testing.T) {
	left := permission.NewBitSet(130)
	left.Set(0)
	left.Set(65)
	left.Set(129)
	right := permission.NewBitSet(130)
	right.Set(65)
	right.Set(128)

	and := left.And(right)
	if and.Has(0) || !and.Has(65) || and.Has(129) || and.Has(128) {
		t.Fatalf("AND mismatch: %#v", []uint64(and))
	}

	or := left.Or(right)
	if !or.Has(0) || !or.Has(65) || !or.Has(128) || !or.Has(129) {
		t.Fatalf("OR mismatch: %#v", []uint64(or))
	}

	andNot := left.AndNot(right)
	if !andNot.Has(0) || andNot.Has(65) || andNot.Has(128) || !andNot.Has(129) {
		t.Fatalf("AND NOT mismatch: %#v", []uint64(andNot))
	}
}

func TestBitSetClearExactly(t *testing.T) {
	b := permission.NewBitSet(64)
	b.Set(0)
	b.Set(63)
	if !b.Has(0) || !b.Has(63) {
		t.Fatal("Set should set bits")
	}
	b.Clear(0)
	if b.Has(0) || !b.Has(63) {
		t.Fatal("Clear should clear only the specified bit")
	}
	b.Clear(63)
	if b.Has(63) || !b.Empty() {
		t.Fatal("clearing all bits should result in empty set")
	}
}

func TestBitSetEmptyExactly(t *testing.T) {
	if !permission.NewBitSet(0).Empty() {
		t.Fatal("nil bitset should be empty")
	}
	if !permission.NewBitSet(64).Empty() {
		t.Fatal("fresh bitset should be empty")
	}
	b := permission.NewBitSet(64)
	b.Set(10)
	if b.Empty() {
		t.Fatal("bitset with set bit should not be empty")
	}
}

func TestBitSetSetAndHasRejectNegativeIndex(t *testing.T) {
	b := permission.NewBitSet(64)
	b.Set(-1)
	if b.Has(-1) {
		t.Fatal("Has should return false for negative index")
	}
	b.Clear(-1)
	if !b.Empty() {
		t.Fatal("Clear with negative index should be no-op")
	}
}

func TestBitSetHasRejectsOutOfRangeIndex(t *testing.T) {
	b := permission.NewBitSet(64)
	if b.Has(64) {
		t.Fatal("Has should return false for out-of-range index")
	}
}

func TestBitSetOrExtendsLength(t *testing.T) {
	a := permission.NewBitSet(64)
	b := permission.NewBitSet(128)
	b.Set(127)
	result := a.Or(b)
	if !result.Has(127) {
		t.Fatal("Or should extend to larger bitset length")
	}
	if len(result) != len(b) {
		t.Fatalf("Or length=%d want=%d", len(result), len(b))
	}
}

func TestBitSetCloneIndependence(t *testing.T) {
	a := permission.NewBitSet(64)
	a.Set(5)
	clone := a.Clone()
	clone.Clear(5)
	if !a.Has(5) {
		t.Fatal("clone mutation should not affect original")
	}
	a.Set(10)
	if clone.Has(10) {
		t.Fatal("original mutation should not affect clone")
	}
}

func TestBitSetAndShorterOther(t *testing.T) {
	a := permission.NewBitSet(128)
	a.Set(0)
	a.Set(100)
	b := permission.NewBitSet(64)
	b.Set(0)
	result := a.And(b)
	if !result.Has(0) || result.Has(100) {
		t.Fatal("And with shorter other should clear out-of-range bits")
	}
}

func TestBitSetAndNotShorterOther(t *testing.T) {
	a := permission.NewBitSet(128)
	a.Set(0)
	a.Set(100)
	b := permission.NewBitSet(64)
	result := a.AndNot(b)
	if !result.Has(0) || !result.Has(100) {
		t.Fatal("AndNot with shorter other should keep out-of-range bits")
	}
}
