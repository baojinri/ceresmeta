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

package server

import "github.com/CeresDB/horaemeta/pkg/coderr"

var (
	ErrCreateEtcdClient    = coderr.NewCodeError(coderr.Internal, "create etcd etcdCli")
	ErrStartEtcd           = coderr.NewCodeError(coderr.Internal, "start embed etcd")
	ErrStartEtcdTimeout    = coderr.NewCodeError(coderr.Internal, "start etcd server timeout")
	ErrStartServer         = coderr.NewCodeError(coderr.Internal, "start server")
	ErrFlowLimiterNotFound = coderr.NewCodeError(coderr.Internal, "flow limiter not found")
)
