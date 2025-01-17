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

package status

import "sync/atomic"

type Status int32

const (
	StatusWaiting Status = iota
	StatusRunning
	Terminated
)

type ServerStatus struct {
	status Status
}

func NewServerStatus() *ServerStatus {
	return &ServerStatus{
		status: StatusWaiting,
	}
}

func (s *ServerStatus) Set(status Status) {
	atomic.StoreInt32((*int32)(&s.status), int32(status))
}

func (s *ServerStatus) Get() Status {
	return Status(atomic.LoadInt32((*int32)(&s.status)))
}

func (s *ServerStatus) IsHealthy() bool {
	return s.Get() == StatusRunning
}
