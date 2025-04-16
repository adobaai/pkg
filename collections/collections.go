// Package collection provides some useful functions for working with data structures
// that contain multiple elements.
//
// Many languages have their own collection library:
//   - C#: https://learn.microsoft.com/en-us/dotnet/csharp/programming-guide/concepts/collections
//   - Rust: https://doc.rust-lang.org/std/collections/index.html
//   - Swift: https://github.com/apple/swift-collections
//   - Kotlin: https://kotlinlang.org/api/latest/jvm/stdlib/kotlin.collections/
//   - Python3: https://docs.python.org/3/library/collections.html
package collections

// Filter iterates over items, returning an array of all items predicate returns truthy for.
func Filter[V any](items []V, predicate func(it V) bool) []V {
	result := make([]V, 0, len(items))
	for _, it := range items {
		if predicate(it) {
			result = append(result, it)
		}
	}
	return result
}

// Map returns a slice containing the results of applying the given transform function
// to each item in the original slice.
func Map[T, R any](items []T, transform func(it T) R) []R {
	res := make([]R, len(items))
	for i, item := range items {
		res[i] = transform(item)
	}
	return res
}
