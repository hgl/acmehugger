package stack

type Stack[T any] []T

func (s *Stack[T]) Push(v T) {
	*s = append(*s, v)
}

func (s *Stack[T]) Pop() (v T, ok bool) {
	v, ok = s.Peek()
	if !ok {
		return
	}
	*s = (*s)[:len(*s)-1]
	return
}

func (s *Stack[T]) MustPop() T {
	item := s.MustPeek()
	*s = (*s)[:len(*s)-1]
	return item
}

func (s Stack[T]) Peek() (v T, ok bool) {
	if len(s) == 0 {
		ok = false
		return
	}
	v = s[len(s)-1]
	ok = true
	return
}

func (s Stack[T]) MustPeek() T {
	item, ok := s.Peek()
	if !ok {
		panic("empty stack")
	}
	return item
}

func (s Stack[T]) Depth() int {
	return len(s)
}
