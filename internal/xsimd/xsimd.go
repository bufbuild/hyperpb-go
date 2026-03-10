package xsimd

import "buf.build/go/hyperpb/internal/xunsafe/layout"

// ToSlice converts a vector into a slice.
func ToSlice[V vector[T], T any](v V) []T {
	n := layout.Size[V]() / layout.Size[T]()
	out := make([]T, n)
	v.StoreSlice(out)
	return out
}

type vector[T any] interface {
	StoreSlice([]T)
}

type format[V vector[T], T any] struct {
	v V
}
