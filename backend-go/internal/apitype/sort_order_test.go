package apitype

import (
	"reflect"
	"testing"
)

func TestSortOrderAndImmutablePage(t *testing.T) {
	for _, value := range []string{"", "asc", "desc"} {
		if _, err := ParseSortOrder(value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ParseSortOrder("DESC;bad"); err == nil {
		t.Fatal("invalid order accepted")
	}
	rows := []int{1, 2, 3, 4, 5}
	if got := CopyOrderedRange(rows, 0, 2, SortDescending); !reflect.DeepEqual(got, []int{5, 4}) {
		t.Fatal(got)
	}
	if got := CopyOrderedRange(rows, 4, 5, SortDescending); !reflect.DeepEqual(got, []int{1}) {
		t.Fatal(got)
	}
	if got := CopyOrderedRange(rows, 0, 2, SortAscending); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatal(got)
	}
	copy := CopyOrderedRange(rows, 0, 2, SortAscending)
	copy[0] = 99
	if rows[0] != 1 {
		t.Fatal("source changed")
	}
	if len(CopyOrderedRange(rows, 0, 0, SortDescending)) != 0 {
		t.Fatal("nonempty page")
	}
}
