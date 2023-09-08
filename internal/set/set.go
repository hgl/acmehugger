package set

type Set[T comparable] map[T]struct{}

func (s Set[T]) Add(item T) {
	s[item] = struct{}{}
}

func (s Set[T]) AddNew(item T) bool {
	_, exist := s[item]
	s[item] = struct{}{}
	return !exist
}

func (s Set[T]) Clear() {
	for k := range s {
		delete(s, k)
	}
}

func (s1 Set[T]) Equal(s2 Set[T]) bool {
	if len(s1) != len(s2) {
		return false
	}
	for v := range s1 {
		if _, ok := s2[v]; !ok {
			return false
		}
	}
	return true
}

func NewSet[T comparable](items ...T) Set[T] {
	m := map[T]struct{}{}
	for _, item := range items {
		m[item] = struct{}{}
	}
	return m
}

func EqualSet[T comparable](s1 []T, s2 []T) bool {
	return NewSet(s1...).Equal(NewSet(s2...))
}
