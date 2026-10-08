//go:build go1.26

package locking

import "sync"

type Mutex = sync.Mutex
type RWMutex = sync.RWMutex
