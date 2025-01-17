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

package test

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/CeresDB/horaemeta/server/cluster"
	"github.com/CeresDB/horaemeta/server/cluster/metadata"
	"github.com/CeresDB/horaemeta/server/coordinator/eventdispatch"
	"github.com/CeresDB/horaemeta/server/coordinator/procedure"
	"github.com/CeresDB/horaemeta/server/coordinator/scheduler"
	"github.com/CeresDB/horaemeta/server/coordinator/scheduler/nodepicker"
	"github.com/CeresDB/horaemeta/server/etcdutil"
	"github.com/CeresDB/horaemeta/server/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	TestTableName0                     = "table0"
	TestTableName1                     = "table1"
	TestSchemaName                     = "TestSchemaName"
	TestRootPath                       = "/rootPath"
	DefaultIDAllocatorStep             = 20
	ClusterName                        = "ceresdbCluster1"
	DefaultNodeCount                   = 2
	DefaultShardTotal                  = 4
	DefaultSchedulerOperator           = true
	DefaultTopologyType                = "static"
	DefaultProcedureExecutingBatchSize = math.MaxUint32
)

type MockDispatch struct{}

func (m MockDispatch) OpenShard(_ context.Context, _ string, _ eventdispatch.OpenShardRequest) error {
	return nil
}

func (m MockDispatch) CloseShard(_ context.Context, _ string, _ eventdispatch.CloseShardRequest) error {
	return nil
}

func (m MockDispatch) CreateTableOnShard(_ context.Context, _ string, _ eventdispatch.CreateTableOnShardRequest) (uint64, error) {
	return 0, nil
}

func (m MockDispatch) DropTableOnShard(_ context.Context, _ string, _ eventdispatch.DropTableOnShardRequest) (uint64, error) {
	return 0, nil
}

func (m MockDispatch) OpenTableOnShard(_ context.Context, _ string, _ eventdispatch.OpenTableOnShardRequest) error {
	return nil
}

func (m MockDispatch) CloseTableOnShard(_ context.Context, _ string, _ eventdispatch.CloseTableOnShardRequest) error {
	return nil
}

type MockStorage struct{}

func (m MockStorage) CreateOrUpdate(_ context.Context, _ procedure.Meta) error {
	return nil
}

func (m MockStorage) CreateOrUpdateWithTTL(_ context.Context, _ procedure.Meta, _ int64) error {
	return nil
}

func (m MockStorage) List(_ context.Context, _ procedure.Kind, _ int) ([]*procedure.Meta, error) {
	return nil, nil
}

func (m MockStorage) Delete(_ context.Context, _ procedure.Kind, _ uint64) error {
	return nil
}

func (m MockStorage) MarkDeleted(_ context.Context, _ procedure.Kind, _ uint64) error {
	return nil
}

func NewTestStorage(_ *testing.T) procedure.Storage {
	return MockStorage{}
}

type MockIDAllocator struct{}

func (m MockIDAllocator) Alloc(_ context.Context) (uint64, error) {
	return 0, nil
}

func (m MockIDAllocator) Collect(_ context.Context, _ uint64) error {
	return nil
}

// InitEmptyCluster will return a cluster that has created shards and nodes, but it does not have any shard node mapping.
func InitEmptyCluster(ctx context.Context, t *testing.T) *cluster.Cluster {
	re := require.New(t)

	_, client, _ := etcdutil.PrepareEtcdServerAndClient(t)
	clusterStorage := storage.NewStorageWithEtcdBackend(client, TestRootPath, storage.Options{
		MaxScanLimit: 100, MinScanLimit: 10, MaxOpsPerTxn: 10,
	})

	logger := zap.NewNop()

	clusterMetadata := metadata.NewClusterMetadata(logger, storage.Cluster{
		ID:                          0,
		Name:                        ClusterName,
		MinNodeCount:                DefaultNodeCount,
		ShardTotal:                  DefaultShardTotal,
		TopologyType:                DefaultTopologyType,
		ProcedureExecutingBatchSize: DefaultProcedureExecutingBatchSize,
		CreatedAt:                   0,
		ModifiedAt:                  0,
	}, clusterStorage, client, TestRootPath, DefaultIDAllocatorStep)

	err := clusterMetadata.Init(ctx)
	re.NoError(err)

	err = clusterMetadata.Load(ctx)
	re.NoError(err)

	c, err := cluster.NewCluster(logger, clusterMetadata, client, TestRootPath)
	re.NoError(err)

	_, _, err = c.GetMetadata().GetOrCreateSchema(ctx, TestSchemaName)
	re.NoError(err)

	lastTouchTime := time.Now().UnixMilli()
	for i := 0; i < DefaultNodeCount; i++ {
		node := storage.Node{
			Name:          fmt.Sprintf("node%d", i),
			NodeStats:     storage.NewEmptyNodeStats(),
			LastTouchTime: uint64(lastTouchTime),
			State:         storage.NodeStateUnknown,
		}
		err = c.GetMetadata().RegisterNode(ctx, metadata.RegisteredNode{
			Node:       node,
			ShardInfos: nil,
		})
		re.NoError(err)
	}

	return c
}

func InitEmptyClusterWithConfig(ctx context.Context, t *testing.T, shardNumber int, nodeNumber int) *cluster.Cluster {
	re := require.New(t)

	_, client, _ := etcdutil.PrepareEtcdServerAndClient(t)
	clusterStorage := storage.NewStorageWithEtcdBackend(client, TestRootPath, storage.Options{
		MaxScanLimit: 100, MinScanLimit: 10, MaxOpsPerTxn: 32,
	})

	logger := zap.NewNop()

	clusterMetadata := metadata.NewClusterMetadata(logger, storage.Cluster{
		ID:                          0,
		Name:                        ClusterName,
		MinNodeCount:                uint32(nodeNumber),
		ShardTotal:                  uint32(shardNumber),
		TopologyType:                DefaultTopologyType,
		ProcedureExecutingBatchSize: DefaultProcedureExecutingBatchSize,
		CreatedAt:                   0,
		ModifiedAt:                  0,
	}, clusterStorage, client, TestRootPath, DefaultIDAllocatorStep)

	err := clusterMetadata.Init(ctx)
	re.NoError(err)

	err = clusterMetadata.Load(ctx)
	re.NoError(err)

	c, err := cluster.NewCluster(logger, clusterMetadata, client, TestRootPath)
	re.NoError(err)

	_, _, err = c.GetMetadata().GetOrCreateSchema(ctx, TestSchemaName)
	re.NoError(err)

	lastTouchTime := time.Now().UnixMilli()
	for i := 0; i < nodeNumber; i++ {
		node := storage.Node{
			Name:          fmt.Sprintf("node%d", i),
			NodeStats:     storage.NewEmptyNodeStats(),
			LastTouchTime: uint64(lastTouchTime),
			State:         storage.NodeStateUnknown,
		}
		err = c.GetMetadata().RegisterNode(ctx, metadata.RegisteredNode{
			Node:       node,
			ShardInfos: []metadata.ShardInfo{},
		})
		re.NoError(err)
	}

	return c
}

// InitPrepareCluster will return a cluster that has created shards and nodes, and cluster state is prepare.
func InitPrepareCluster(ctx context.Context, t *testing.T) *cluster.Cluster {
	re := require.New(t)
	c := InitEmptyCluster(ctx, t)

	err := c.GetMetadata().UpdateClusterView(ctx, storage.ClusterStatePrepare, []storage.ShardNode{})
	re.NoError(err)

	return c
}

// InitStableCluster will return a cluster that has created shards and nodes, and shards have been assigned to existing nodes.
func InitStableCluster(ctx context.Context, t *testing.T) *cluster.Cluster {
	re := require.New(t)
	c := InitEmptyCluster(ctx, t)
	snapshot := c.GetMetadata().GetClusterSnapshot()
	shardNodes := make([]storage.ShardNode, 0, DefaultShardTotal)
	for _, shardView := range snapshot.Topology.ShardViewsMapping {
		selectNodeIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(snapshot.RegisteredNodes))))
		re.NoError(err)
		shardNodes = append(shardNodes, storage.ShardNode{
			ID:        shardView.ShardID,
			ShardRole: storage.ShardRoleLeader,
			NodeName:  snapshot.RegisteredNodes[selectNodeIdx.Int64()].Node.Name,
		})
	}

	err := c.GetMetadata().UpdateClusterView(ctx, storage.ClusterStateStable, shardNodes)
	re.NoError(err)

	return c
}

func InitStableClusterWithConfig(ctx context.Context, t *testing.T, nodeNumber int, shardNumber int) *cluster.Cluster {
	re := require.New(t)
	c := InitEmptyClusterWithConfig(ctx, t, shardNumber, nodeNumber)
	snapshot := c.GetMetadata().GetClusterSnapshot()
	shardNodes := make([]storage.ShardNode, 0, DefaultShardTotal)
	nodePicker := nodepicker.NewConsistentUniformHashNodePicker(zap.NewNop())
	var unAssignedShardIDs []storage.ShardID
	for i := 0; i < shardNumber; i++ {
		unAssignedShardIDs = append(unAssignedShardIDs, storage.ShardID(i))
	}
	shardNodeMapping, err := nodePicker.PickNode(ctx, nodepicker.Config{
		NumTotalShards:    uint32(shardNumber),
		ShardAffinityRule: map[storage.ShardID]scheduler.ShardAffinity{},
	}, unAssignedShardIDs, snapshot.RegisteredNodes)
	re.NoError(err)

	for shardID, node := range shardNodeMapping {
		shardNodes = append(shardNodes, storage.ShardNode{
			ID:        shardID,
			ShardRole: storage.ShardRoleLeader,
			NodeName:  node.Node.Name,
		})
	}

	err = c.GetMetadata().UpdateClusterView(ctx, storage.ClusterStateStable, shardNodes)
	re.NoError(err)

	return c
}
