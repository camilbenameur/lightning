package utils

import "github.com/nrednav/cuid2"

func NewId() string {
	return cuid2.Generate()
}
