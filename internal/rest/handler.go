package rest

import "lightning/internal/runner"

type handler struct {
	runner *runner.Runner
}

func newHandler() handler {
	return handler{runner: runner.NewRunner()}
}
