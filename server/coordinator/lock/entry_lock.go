/*
 * Copyright 2022 The HoraeDB Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package lock

import (
	"fmt"
	"sync"
)

type EntryLock struct {
	lock       sync.Mutex
	entryLocks map[uint64]struct{}
}

func NewEntryLock(initCapacity int) EntryLock {
	return EntryLock{
		lock:       sync.Mutex{},
		entryLocks: make(map[uint64]struct{}, initCapacity),
	}
}

func (l *EntryLock) TryLock(locks []uint64) bool {
	l.lock.Lock()
	defer l.lock.Unlock()

	for _, lock := range locks {
		_, exists := l.entryLocks[lock]
		if exists {
			return false
		}
	}

	for _, lock := range locks {
		l.entryLocks[lock] = struct{}{}
	}

	return true
}

func (l *EntryLock) UnLock(locks []uint64) {
	l.lock.Lock()
	defer l.lock.Unlock()

	for _, lock := range locks {
		_, exists := l.entryLocks[lock]
		if !exists {
			panic(fmt.Sprintf("try to unlock nonexistent lock, exists locks:%v, unlock locks:%v", l.entryLocks, locks))
		}
	}

	for _, lock := range locks {
		delete(l.entryLocks, lock)
	}
}
