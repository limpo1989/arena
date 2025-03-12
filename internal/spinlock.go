package internal

import (
	"runtime"
	"sync/atomic"
)

type SpinLock int32

func (sl *SpinLock) Lock() {
	var backoff = 1
	const maxBackoff = 16

	for {
		// try lock
		if !atomic.CompareAndSwapInt32((*int32)(sl), 0, 1) {
			// Leverage the exponential backoff algorithm, see https://en.wikipedia.org/wiki/Exponential_backoff.
			for i := 0; i < backoff; i++ {
				runtime.Gosched()
			}
			if backoff < maxBackoff {
				backoff <<= 1
			}
			// try again
			continue
		}

		// get lock
		break
	}
}

func (sl *SpinLock) Unlock() {
	atomic.StoreInt32((*int32)(sl), 0)
}
