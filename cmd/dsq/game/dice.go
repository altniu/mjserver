package game

import "math/rand"

type dice struct {
	dice1 int
}

func newDice() *dice {
	return &dice{}
}

func (d *dice) random() {
	d.dice1 = rand.Intn(2)
