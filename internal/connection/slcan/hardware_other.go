//go:build !linux

package slcan

import (
	"fmt"
	"sync"
)

func (scc *SLCanConnectionClient) Open() error {
	return fmt.Errorf("slcan is only supported on Linux")
}

func (scc *SLCanConnectionClient) Close() error {
	return fmt.Errorf("slcan is only supported on Linux")
}

func (scc *SLCanConnectionClient) Receive(_ *sync.WaitGroup) {}
