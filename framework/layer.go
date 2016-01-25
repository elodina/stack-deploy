package framework

import "fmt"

const (
	LayerZone = iota
	LayerCluster
	LayerDataCenter
	LayerStack
)

type Layer struct {
	Level int
	Stack *Stack
}

type Merger interface {
	Merge(*Stack)
	GetRunner() Runner
}

func NewLayer(stack *Stack) *Layer {
	switch stack.Layer {
	case LayerZone:
		return NewZone(stack)
	case LayerCluster:
		return NewCluster(stack)
	case LayerDataCenter:
		return NewDataCenter(stack)
	}
	return nil
}

func NewDataCenter(stack *Stack) *Layer {
	return &Layer{Level: LayerDataCenter, Stack: stack}
}

func NewCluster(stack *Stack) *Layer {
	return &Layer{Level: LayerCluster, Stack: stack}
}

func NewZone(stack *Stack) *Layer {
	return &Layer{Level: LayerZone, Stack: stack}
}

// you can merge only lower level layers
// Stack.Merge(DataCenter)
// DataCenter.Merge(Cluster))
// Cluster.Merge(Zone)
func (l *Layer) Merge(other *Layer) error {
	if l.Level < other.Level {
		return fmt.Errorf("Can't merge layer level %d with level %d", l.Level, other.Level)
	}
	l.Stack.Merge(other.Stack)
	return nil
}
