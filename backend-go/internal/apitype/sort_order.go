package apitype

import "fmt"

type SortOrder string

const (
	SortAscending  SortOrder = "asc"
	SortDescending SortOrder = "desc"
)

func ParseSortOrder(value string) (SortOrder, error) {
	switch value {
	case "", "asc":
		return SortAscending, nil
	case "desc":
		return SortDescending, nil
	default:
		return "", fmt.Errorf("排序参数必须为 asc 或 desc")
	}
}

// CopyOrderedRange reads a page of an ascending snapshot without changing it.
// The caller validates and clamps start/end to [0, len(rows)].
func CopyOrderedRange[T any](rows []T, start, end int, order SortOrder) []T {
	result := make([]T, 0, end-start)
	for i := start; i < end; i++ {
		index := i
		if order == SortDescending {
			index = len(rows) - 1 - i
		}
		result = append(result, rows[index])
	}
	return result
}
