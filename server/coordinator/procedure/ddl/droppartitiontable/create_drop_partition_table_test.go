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

package droppartitiontable_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/CeresDB/horaedbproto/golang/pkg/clusterpb"
	"github.com/CeresDB/horaedbproto/golang/pkg/metaservicepb"
	"github.com/CeresDB/horaemeta/server/cluster"
	"github.com/CeresDB/horaemeta/server/cluster/metadata"
	"github.com/CeresDB/horaemeta/server/coordinator"
	"github.com/CeresDB/horaemeta/server/coordinator/eventdispatch"
	"github.com/CeresDB/horaemeta/server/coordinator/procedure"
	"github.com/CeresDB/horaemeta/server/coordinator/procedure/ddl/createpartitiontable"
	"github.com/CeresDB/horaemeta/server/coordinator/procedure/ddl/droppartitiontable"
	"github.com/CeresDB/horaemeta/server/coordinator/procedure/test"
	"github.com/CeresDB/horaemeta/server/storage"
	"github.com/stretchr/testify/require"
)

func TestCreateAndDropPartitionTable(t *testing.T) {
	re := require.New(t)
	ctx := context.Background()
	dispatch := test.MockDispatch{}
	c := test.InitStableCluster(ctx, t)
	s := test.NewTestStorage(t)

	shardNode := c.GetMetadata().GetClusterSnapshot().Topology.ClusterView.ShardNodes[0]

	shardPicker := coordinator.NewLeastTableShardPicker()

	testTableNum := 8
	testSubTableNum := 4

	// Create table.
	for i := 0; i < testTableNum; i++ {
		tableName := fmt.Sprintf("%s_%d", test.TestTableName0, i)
		subTableNames := genSubTables(tableName, testSubTableNum)
		testCreatePartitionTable(ctx, t, dispatch, c, s, shardPicker, shardNode.NodeName, tableName, subTableNames)
	}

	// Check get table.
	for i := 0; i < testTableNum; i++ {
		tableName := fmt.Sprintf("%s_%d", test.TestTableName0, i)
		table := checkTable(t, c, tableName, true)
		re.Equal(table.PartitionInfo.Info != nil, true)
		subTableNames := genSubTables(tableName, testSubTableNum)
		for _, subTableName := range subTableNames {
			checkTable(t, c, subTableName, true)
		}
	}

	// Drop table.
	for i := 0; i < testTableNum; i++ {
		tableName := fmt.Sprintf("%s_%d", test.TestTableName0, i)
		subTableNames := genSubTables(tableName, testSubTableNum)
		testDropPartitionTable(t, dispatch, c, s, shardNode.NodeName, tableName, subTableNames)
	}

	// Check table not exists.
	for i := 0; i < testTableNum; i++ {
		tableName := fmt.Sprintf("%s_%d", test.TestTableName0, i)
		checkTable(t, c, tableName, false)
		subTableNames := genSubTables(tableName, testSubTableNum)
		for _, subTableName := range subTableNames {
			checkTable(t, c, subTableName, false)
		}
	}
}

func testCreatePartitionTable(ctx context.Context, t *testing.T, dispatch eventdispatch.Dispatch, c *cluster.Cluster, s procedure.Storage, shardPicker coordinator.ShardPicker, nodeName string, tableName string, subTableNames []string) {
	re := require.New(t)

	partitionInfo := clusterpb.PartitionInfo{
		Info: nil,
	}
	request := &metaservicepb.CreateTableRequest{
		Header: &metaservicepb.RequestHeader{
			Node:        nodeName,
			ClusterName: test.ClusterName,
		},
		PartitionTableInfo: &metaservicepb.PartitionTableInfo{
			SubTableNames: subTableNames,
			PartitionInfo: &partitionInfo,
		},
		SchemaName: test.TestSchemaName,
		Name:       tableName,
	}

	subTableShards, err := shardPicker.PickShards(ctx, c.GetMetadata().GetClusterSnapshot(), len(request.GetPartitionTableInfo().SubTableNames))
	re.NoError(err)

	shardNodesWithVersion := make([]metadata.ShardNodeWithVersion, 0, len(subTableShards))
	for _, subTableShard := range subTableShards {
		shardView, exists := c.GetMetadata().GetClusterSnapshot().Topology.ShardViewsMapping[subTableShard.ID]
		re.True(exists)
		shardNodesWithVersion = append(shardNodesWithVersion, metadata.ShardNodeWithVersion{
			ShardInfo: metadata.ShardInfo{
				ID:      shardView.ShardID,
				Role:    subTableShard.ShardRole,
				Version: shardView.Version,
				Status:  storage.ShardStatusUnknown,
			},
			ShardNode: subTableShard,
		})
	}

	procedure, err := createpartitiontable.NewProcedure(createpartitiontable.ProcedureParams{
		ID:              0,
		ClusterMetadata: c.GetMetadata(),
		ClusterSnapshot: c.GetMetadata().GetClusterSnapshot(),
		Dispatch:        dispatch,
		Storage:         s,
		SourceReq:       request,
		SubTablesShards: shardNodesWithVersion,
		OnSucceeded: func(_ metadata.CreateTableResult) error {
			return nil
		},
		OnFailed: func(err error) error {
			return nil
		},
	})
	re.NoError(err)

	err = procedure.Start(ctx)
	re.NoError(err)
}

func testDropPartitionTable(t *testing.T, dispatch eventdispatch.Dispatch, c *cluster.Cluster, s procedure.Storage, nodeName string, tableName string, subTableNames []string) {
	re := require.New(t)
	// Create DropPartitionTableProcedure to drop table.
	partitionTableInfo := &metaservicepb.PartitionTableInfo{
		PartitionInfo: nil,
		SubTableNames: subTableNames,
	}
	req := droppartitiontable.ProcedureParams{
		ID: uint64(1), Dispatch: dispatch, ClusterMetadata: c.GetMetadata(), ClusterSnapshot: c.GetMetadata().GetClusterSnapshot(), SourceReq: &metaservicepb.DropTableRequest{
			Header: &metaservicepb.RequestHeader{
				Node:        nodeName,
				ClusterName: test.ClusterName,
			},
			SchemaName:         test.TestSchemaName,
			Name:               tableName,
			PartitionTableInfo: partitionTableInfo,
		}, OnSucceeded: func(_ metadata.TableInfo) error {
			return nil
		}, OnFailed: func(_ error) error {
			return nil
		}, Storage: s,
	}

	procedure, ok, err := droppartitiontable.NewProcedure(req)
	re.NoError(err)
	re.True(ok)
	err = procedure.Start(context.Background())
	re.NoError(err)
}

func genSubTables(tableName string, tableNum int) []string {
	var subTableNames []string
	for j := 0; j < tableNum; j++ {
		subTableNames = append(subTableNames, fmt.Sprintf("%s_%d", tableName, j))
	}
	return subTableNames
}

func checkTable(t *testing.T, c *cluster.Cluster, tableName string, exist bool) storage.Table {
	re := require.New(t)
	table, b, err := c.GetMetadata().GetTable(test.TestSchemaName, tableName)
	re.NoError(err)
	re.Equal(b, exist)
	return table
}
