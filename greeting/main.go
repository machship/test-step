package main

import (
	"github.com/machship/step-essentials/io"
)

func main() {
	inputs := io.GetInputs()

	name, ok := inputs["name"].(string)
	if !ok || name == "" {
		name = "World"
	}
	msg := getMessage(name)

	io.SetOutputs(map[string]any{
		"message": msg,
	})
}
