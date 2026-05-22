package internal

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpinLockBasic(t *testing.T) {
	var sl SpinLock
	sl.Lock()
	sl.Unlock()
}

func TestSpinLockContention(t *testing.T) {
	var sl SpinLock
	var counter int64

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				sl.Lock()
				atomic.AddInt64(&counter, 1)
				sl.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(100*1000), atomic.LoadInt64(&counter))
}
