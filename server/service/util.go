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

package service

import (
	"context"
	"net/url"
	"strings"

	"github.com/CeresDB/horaemeta/pkg/coderr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	ErrParseURL = coderr.NewCodeError(coderr.Internal, "parse url")
	ErrGRPCDial = coderr.NewCodeError(coderr.Internal, "grpc dial")
)

// GetClientConn returns a gRPC client connection.
func GetClientConn(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	opt := grpc.WithTransportCredentials(insecure.NewCredentials())

	host := addr
	if strings.HasPrefix(addr, "http") {
		u, err := url.Parse(addr)
		if err != nil {
			return nil, ErrParseURL.WithCause(err)
		}
		host = u.Host
	}

	cc, err := grpc.DialContext(ctx, host, opt)
	if err != nil {
		return nil, ErrGRPCDial.WithCause(err)
	}
	return cc, nil
}
