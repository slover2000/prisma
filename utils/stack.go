package utils

import (
	"container/list"
)

type (
	// Stack interface
	Stack interface {
		Push(value interface{})
		Pop() interface{}
		Peak() interface{}
		Len() int
		Empty() bool
	}

	stack struct {
		list *list.List
	}
)

// NewStack create a stack
func NewStack() Stack {
	return &stack{
		list: list.New(),
	}
}

func (s *stack) Push(value interface{}) {
	s.list.PushBack(value)
}

func (s *stack) Pop() interface{} {
	e := s.list.Back()
	if e != nil {
		s.list.Remove(e)
		return e.Value
	}
	return nil
}

func (s *stack) Peak() interface{} {
	e := s.list.Back()
	if e != nil {
		return e.Value
	}

	return nil
}

func (s *stack) Len() int {
	return s.list.Len()
}

func (s *stack) Empty() bool {
	return s.list.Len() == 0
}
