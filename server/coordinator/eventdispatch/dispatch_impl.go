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

package eventdispatch

import (
	"context"
	"sync"

	"github.com/CeresDB/horaedbproto/golang/pkg/metaeventpb"
	"github.com/CeresDB/horaemeta/pkg/coderr"
	"github.com/CeresDB/horaemeta/server/cluster/metadata"
	"github.com/CeresDB/horaemeta/server/service"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

var ErrDispatch = coderr.NewCodeError(coderr.Internal, "event dispatch failed")

type DispatchImpl struct {
	conns sync.Map
}

func NewDispatchImpl() *DispatchImpl {
	return &DispatchImpl{
		conns: sync.Map{},
	}
}

func (d *DispatchImpl) OpenShard(ctx context.Context, addr string, request OpenShardRequest) error {
	client, err := d.getMetaEventClient(ctx, addr)
	if err != nil {
		return err
	}
	resp, err := client.OpenShard(ctx, &metaeventpb.OpenShardRequest{
		Shard: metadata.ConvertShardsInfoToPB(request.Shard),
	})
	if err != nil {
		return errors.WithMessagef(err, "open shard, addr:%s, request:%v", addr, request)
	}
	if resp.GetHeader().Code != 0 {
		return ErrDispatch.WithCausef("open shard, addr:%s, request:%v, err:%s", addr, request, resp.GetHeader().GetError())
	}
	return nil
}

func (d *DispatchImpl) CloseShard(ctx context.Context, addr string, request CloseShardRequest) error {
	client, err := d.getMetaEventClient(ctx, addr)
	if err != nil {
		return err
	}
	resp, err := client.CloseShard(ctx, &metaeventpb.CloseShardRequest{
		ShardId: request.ShardID,
	})
	if err != nil {
		return errors.WithMessagef(err, "close shard, addr:%s, request:%v", addr, request)
	}
	if resp.GetHeader().Code != 0 {
		return ErrDispatch.WithCausef("close shard, addr:%s, request:%v, err:%s", addr, request, resp.GetHeader().GetError())
	}
	return nil
}

func (d *DispatchImpl) CreateTableOnShard(ctx context.Context, addr string, request CreateTableOnShardRequest) (uint64, error) {
	client, err := d.getMetaEventClient(ctx, addr)
	if err != nil {
		return 0, err
	}
	resp, err := client.CreateTableOnShard(ctx, convertCreateTableOnShardRequestToPB(request))
	if err != nil {
		return 0, errors.WithMessagef(err, "create table on shard, addr:%s, request:%v", addr, request)
	}
	if resp.GetHeader().Code != 0 {
		return 0, ErrDispatch.WithCausef("create table on shard, addr:%s, request:%v, err:%s", addr, request, resp.GetHeader().GetError())
	}
	return resp.GetLatestShardVersion(), nil
}

func (d *DispatchImpl) DropTableOnShard(ctx context.Context, addr string, request DropTableOnShardRequest) (uint64, error) {
	client, err := d.getMetaEventClient(ctx, addr)
	if err != nil {
		return 0, err
	}
	resp, err := client.DropTableOnShard(ctx, convertDropTableOnShardRequestToPB(request))
	if err != nil {
		return 0, errors.WithMessagef(err, "drop table on shard, addr:%s, request:%v", addr, request)
	}
	if resp.GetHeader().Code != 0 {
		return 0, ErrDispatch.WithCausef("drop table on shard, addr:%s, request:%v, err:%s", addr, request, resp.GetHeader().GetError())
	}
	return resp.GetLatestShardVersion(), nil
}

func (d *DispatchImpl) OpenTableOnShard(ctx context.Context, addr string, request OpenTableOnShardRequest) error {
	client, err := d.getMetaEventClient(ctx, addr)
	if err != nil {
		return err
	}

	resp, err := client.OpenTableOnShard(ctx, convertOpenTableOnShardRequestToPB(request))
	if err != nil {
		return errors.WithMessagef(err, "open table on shard, addr:%s, request:%v", addr, request)
	}
	if resp.GetHeader().Code != 0 {
		return ErrDispatch.WithCausef("open table on shard, addr:%s, request:%v, err:%s", addr, request, resp.GetHeader().GetError())
	}
	return nil
}

func (d *DispatchImpl) CloseTableOnShard(ctx context.Context, addr string, request CloseTableOnShardRequest) error {
	client, err := d.getMetaEventClient(ctx, addr)
	if err != nil {
		return err
	}

	resp, err := client.CloseTableOnShard(ctx, convertCloseTableOnShardRequestToPB(request))
	if err != nil {
		return errors.WithMessagef(err, "close table on shard, addr:%s, request:%v", addr, request)
	}
	if resp.GetHeader().Code != 0 {
		return ErrDispatch.WithCausef("close table on shard, addr:%s, request:%v, err:%s", addr, request, resp.GetHeader().GetError())
	}
	return nil
}

func (d *DispatchImpl) getGrpcClient(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	client, ok := d.conns.Load(addr)
	if !ok {
		cc, err := service.GetClientConn(ctx, addr)
		if err != nil {
			return nil, err
		}
		client = cc
		d.conns.Store(addr, cc)
	}
	return client.(*grpc.ClientConn), nil
}

func (d *DispatchImpl) getMetaEventClient(ctx context.Context, addr string) (metaeventpb.MetaEventServiceClient, error) {
	client, err := d.getGrpcClient(ctx, addr)
	if err != nil {
		return nil, errors.WithMessagef(err, "get meta event client, addr:%s", addr)
	}
	return metaeventpb.NewMetaEventServiceClient(client), nil
}

func convertCreateTableOnShardRequestToPB(request CreateTableOnShardRequest) *metaeventpb.CreateTableOnShardRequest {
	return &metaeventpb.CreateTableOnShardRequest{
		UpdateShardInfo:  convertUpdateShardInfoToPB(request.UpdateShardInfo),
		TableInfo:        metadata.ConvertTableInfoToPB(request.TableInfo),
		EncodedSchema:    request.EncodedSchema,
		Engine:           request.Engine,
		CreateIfNotExist: request.CreateIfNotExist,
		Options:          request.Options,
	}
}

func convertDropTableOnShardRequestToPB(request DropTableOnShardRequest) *metaeventpb.DropTableOnShardRequest {
	return &metaeventpb.DropTableOnShardRequest{
		UpdateShardInfo: convertUpdateShardInfoToPB(request.UpdateShardInfo),
		TableInfo:       metadata.ConvertTableInfoToPB(request.TableInfo),
	}
}

func convertCloseTableOnShardRequestToPB(request CloseTableOnShardRequest) *metaeventpb.CloseTableOnShardRequest {
	return &metaeventpb.CloseTableOnShardRequest{
		UpdateShardInfo: convertUpdateShardInfoToPB(request.UpdateShardInfo),
		TableInfo:       metadata.ConvertTableInfoToPB(request.TableInfo),
	}
}

func convertOpenTableOnShardRequestToPB(request OpenTableOnShardRequest) *metaeventpb.OpenTableOnShardRequest {
	return &metaeventpb.OpenTableOnShardRequest{
		UpdateShardInfo: convertUpdateShardInfoToPB(request.UpdateShardInfo),
		TableInfo:       metadata.ConvertTableInfoToPB(request.TableInfo),
	}
}

func convertUpdateShardInfoToPB(updateShardInfo UpdateShardInfo) *metaeventpb.UpdateShardInfo {
	return &metaeventpb.UpdateShardInfo{
		CurrShardInfo: metadata.ConvertShardsInfoToPB(updateShardInfo.CurrShardInfo),
	}
}
