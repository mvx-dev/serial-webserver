package main

import (
	"testing"
	"time"
)

func TestTime(t *testing.T) {
	t.Log(time.Now().Format(format))
}
