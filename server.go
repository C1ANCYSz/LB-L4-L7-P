package main

import (
	"sync/atomic"
)

type Server struct {
	url         string
	up          atomic.Bool
	connections atomic.Int64
}
