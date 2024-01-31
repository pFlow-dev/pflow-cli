package examples

import "github.com/pflow-dev/go-metamodel/v2/model"

var (
	ExampleModels = map[string]model.Model{
		"DiningPhilosophers": DiningPhilosophers.ToModel(),
		"InhibitorTest":      InhibitorTest.ToModel(),
		"TicTacToe":          TicTacToe.ToModel(),
	}
)
