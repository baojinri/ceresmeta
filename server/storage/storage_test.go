// Copyright 2022 CeresDB Project Authors. Licensed under Apache-2.0.

package storage

import (
	"context"
	"testing"

	"github.com/CeresDB/ceresdbproto/pkg/clusterpb"
	"github.com/CeresDB/ceresmeta/server/etcdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
)

const (
	defaultRootPath  = "/ceresmeta" // TODO: existed bug when rootPath = "/"
	defaultName1     = "name_1"
	defaultName2     = "name_2"
	defaultName3     = "name_3"
	defaultDesc      = "desc"
	defaultClusterID = 1
	defaultSchemaID  = 1
	defaultVersion   = 0
	defaultNodeName1 = "127.0.0.1:8081"
	defaultNodeName2 = "127.0.0.2:8081"
	defaultNodeName3 = "127.0.0.3:8081"
	defaultNodeName4 = "127.0.0.4:8081"
	defaultNodeName5 = "127.0.0.5:8081"
	defaultCase      = "cause"
)

func TestCluster(t *testing.T) {
	re := require.New(t)
	s := NewStorage(t)
	ctx := context.Background()

	// test create cluster
	clusters := make([]*clusterpb.Cluster, 0)
	for i := 0; i < 20; i++ {
		cluster := &clusterpb.Cluster{Id: uint32(i), Name: defaultName1, MinNodeCount: uint32(i), ReplicationFactor: uint32(i), ShardTotal: uint32(i)}
		cluster, err := s.CreateCluster(ctx, cluster)
		re.NoError(err)
		clusters = append(clusters, cluster)
	}

	// test list clusters
	values, err := s.ListClusters(ctx)
	re.NoError(err)

	for i := 0; i < 20; i++ {
		re.Equal(clusters[i].Id, values[i].Id)
		re.Equal(clusters[i].Name, values[i].Name)
		re.Equal(clusters[i].MinNodeCount, values[i].MinNodeCount)
		re.Equal(clusters[i].ReplicationFactor, values[i].ReplicationFactor)
		re.Equal(clusters[i].CreatedAt, values[i].CreatedAt)
		re.Equal(clusters[i].ShardTotal, values[i].ShardTotal)
	}

	// test put cluster
	cluster := &clusterpb.Cluster{Id: defaultClusterID, Name: defaultName1, MinNodeCount: uint32(1), ReplicationFactor: uint32(1), ShardTotal: uint32(1)}
	err = s.PutCluster(ctx, defaultClusterID, cluster)
	re.NoError(err)

	// test get cluster
	value, err := s.GetCluster(ctx, defaultClusterID)
	re.NoError(err)
	re.Equal(cluster.Id, value.Id)
	re.Equal(cluster.Name, value.Name)
	re.Equal(cluster.MinNodeCount, value.MinNodeCount)
	re.Equal(cluster.ReplicationFactor, value.ReplicationFactor)
	re.Equal(cluster.CreatedAt, value.CreatedAt)
	re.Equal(cluster.ShardTotal, value.ShardTotal)
}

func TestClusterTopology(t *testing.T) {
	re := require.New(t)
	s := NewStorage(t)
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	// test create cluster topology
	clusterTopology := &clusterpb.ClusterTopology{ClusterId: defaultClusterID, DataVersion: defaultVersion, Cause: defaultCase}
	clusterTopology, err := s.CreateClusterTopology(ctx, clusterTopology)
	re.NoError(err)

	// test get cluster topology
	value, err := s.GetClusterTopology(ctx, defaultClusterID)
	re.NoError(err)
	re.Equal(clusterTopology.ClusterId, value.ClusterId)
	re.Equal(clusterTopology.DataVersion, value.DataVersion)
	re.Equal(clusterTopology.Cause, value.Cause)
	re.Equal(clusterTopology.CreatedAt, value.CreatedAt)

	// test put cluster topology
	clusterTopology.DataVersion = uint64(1)
	err = s.PutClusterTopology(ctx, defaultClusterID, defaultVersion, clusterTopology)
	re.NoError(err)

	value, err = s.GetClusterTopology(ctx, defaultClusterID)
	re.NoError(err)
	re.Equal(clusterTopology.ClusterId, value.ClusterId)
	re.Equal(clusterTopology.DataVersion, value.DataVersion)
	re.Equal(clusterTopology.Cause, value.Cause)
	re.Equal(clusterTopology.CreatedAt, value.CreatedAt)
}

func TestSchemes(t *testing.T) {
	re := require.New(t)
	s := NewStorage(t)
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	// test create schema
	schemas := make([]*clusterpb.Schema, 0)
	for i := 0; i < 10; i++ {
		schema := &clusterpb.Schema{Id: uint32(i), ClusterId: defaultClusterID, Name: defaultName1}
		schema, err := s.CreateSchema(ctx, defaultClusterID, schema)
		re.NoError(err)
		schemas = append(schemas, schema)
	}

	// test list schemas
	value, err := s.ListSchemas(ctx, defaultClusterID)
	re.NoError(err)
	for i := 0; i < 10; i++ {
		re.Equal(schemas[i].Id, value[i].Id)
		re.Equal(schemas[i].ClusterId, value[i].ClusterId)
		re.Equal(schemas[i].Name, value[i].Name)
		re.Equal(schemas[i].CreatedAt, value[i].CreatedAt)
	}

	// test put schemas
	for i := 0; i < 10; i++ {
		schemas[i].Name = defaultName2
	}
	err = s.PutSchemas(ctx, defaultClusterID, schemas)
	re.NoError(err)

	value, err = s.ListSchemas(ctx, defaultClusterID)
	re.NoError(err)
	for i := 0; i < 10; i++ {
		re.Equal(schemas[i].Id, value[i].Id)
		re.Equal(schemas[i].ClusterId, value[i].ClusterId)
		re.Equal(schemas[i].Name, value[i].Name)
		re.Equal(schemas[i].CreatedAt, value[i].CreatedAt)
	}
}

func TestTables(t *testing.T) {
	re := require.New(t)
	s := NewStorage(t)
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	// test create tables
	table1 := &clusterpb.Table{Id: uint64(1), Name: defaultName1, SchemaId: defaultSchemaID, ShardId: uint32(1), Desc: defaultDesc}
	table2 := &clusterpb.Table{Id: uint64(2), Name: defaultName2, SchemaId: defaultSchemaID, ShardId: uint32(1), Desc: defaultDesc}
	table3 := &clusterpb.Table{Id: uint64(3), Name: defaultName3, SchemaId: defaultSchemaID, ShardId: uint32(1), Desc: defaultDesc}

	table1, err := s.CreateTable(ctx, defaultClusterID, defaultSchemaID, table1)
	re.NoError(err)
	table2, err = s.CreateTable(ctx, defaultClusterID, defaultSchemaID, table2)
	re.NoError(err)
	table3, err = s.CreateTable(ctx, defaultClusterID, defaultSchemaID, table3)
	re.NoError(err)

	// test get table
	value, exist, err := s.GetTable(ctx, defaultClusterID, defaultSchemaID, defaultName1)
	re.NoError(err)
	re.True(exist)
	re.Equal(table1.Id, value.Id)
	re.Equal(table1.Name, value.Name)
	re.Equal(table1.SchemaId, value.SchemaId)
	re.Equal(table1.ShardId, value.ShardId)
	re.Equal(table1.Desc, value.Desc)
	re.Equal(table1.CreatedAt, value.CreatedAt)

	// test list tables
	tables, err := s.ListTables(ctx, defaultClusterID, defaultSchemaID)
	re.NoError(err)

	re.Equal(table1.Id, tables[0].Id)
	re.Equal(table1.Name, tables[0].Name)
	re.Equal(table1.SchemaId, tables[0].SchemaId)
	re.Equal(table1.ShardId, tables[0].ShardId)
	re.Equal(table1.Desc, tables[0].Desc)
	re.Equal(table1.CreatedAt, tables[0].CreatedAt)

	re.Equal(table2.Id, tables[1].Id)
	re.Equal(table2.Name, tables[1].Name)
	re.Equal(table2.SchemaId, tables[1].SchemaId)
	re.Equal(table2.ShardId, tables[1].ShardId)
	re.Equal(table2.Desc, tables[1].Desc)
	re.Equal(table2.CreatedAt, tables[1].CreatedAt)

	re.Equal(table3.Id, tables[2].Id)
	re.Equal(table3.Name, tables[2].Name)
	re.Equal(table3.SchemaId, tables[2].SchemaId)
	re.Equal(table3.ShardId, tables[2].ShardId)
	re.Equal(table3.Desc, tables[2].Desc)
	re.Equal(table3.CreatedAt, tables[2].CreatedAt)

	// test delete table
	err = s.DeleteTable(ctx, defaultClusterID, defaultSchemaID, defaultName1)
	re.NoError(err)

	value, exist, err = s.GetTable(ctx, defaultClusterID, defaultSchemaID, defaultName1)
	re.NoError(err)
	re.Empty(value)
	re.True(!exist)
}

func TestShardTopologies(t *testing.T) {
	re := require.New(t)
	s := NewStorage(t)
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	// test create shard topologies
	shardTopologies := make([]*clusterpb.ShardTopology, 0)
	shardID := make([]uint32, 0)
	for i := 0; i < 10; i++ {
		shardTopology := &clusterpb.ShardTopology{ShardId: uint32(i), Version: defaultVersion}
		shardTopologies = append(shardTopologies, shardTopology)
		shardID = append(shardID, uint32(i))
	}
	shardTopologies, err := s.CreateShardTopologies(ctx, defaultClusterID, shardTopologies)
	re.NoError(err)

	// test list shard topologies
	value, err := s.ListShardTopologies(ctx, defaultClusterID, shardID)
	re.NoError(err)
	for i := 0; i < 10; i++ {
		re.Equal(shardTopologies[i].ShardId, value[i].ShardId)
		re.Equal(shardTopologies[i].Version, value[i].Version)
		re.Equal(shardTopologies[i].CreatedAt, value[i].CreatedAt)
	}

	// test put shard topologies
	for i := 0; i < 10; i++ {
		shardTopologies[i].Version = 1
	}

	err = s.PutShardTopologies(ctx, defaultClusterID, shardID, defaultVersion, shardTopologies)
	re.NoError(err)

	value, err = s.ListShardTopologies(ctx, defaultClusterID, shardID)
	re.NoError(err)
	for i := 0; i < 10; i++ {
		re.Equal(shardTopologies[i].ShardId, value[i].ShardId)
		re.Equal(shardTopologies[i].Version, value[i].Version)
		re.Equal(shardTopologies[i].CreatedAt, value[i].CreatedAt)
	}
}

func TestNodes(t *testing.T) {
	re := require.New(t)
	s := NewStorage(t)
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	// test create node
	node1 := &clusterpb.Node{Name: defaultNodeName1}
	node1, err := s.CreateOrUpdateNode(ctx, defaultClusterID, node1)
	re.NoError(err)

	node2 := &clusterpb.Node{Name: defaultNodeName2}
	node2, err = s.CreateOrUpdateNode(ctx, defaultClusterID, node2)
	re.NoError(err)

	node3 := &clusterpb.Node{Name: defaultNodeName3}
	node3, err = s.CreateOrUpdateNode(ctx, defaultClusterID, node3)
	re.NoError(err)

	node4 := &clusterpb.Node{Name: defaultNodeName4}
	node4, err = s.CreateOrUpdateNode(ctx, defaultClusterID, node4)
	re.NoError(err)

	node5 := &clusterpb.Node{Name: defaultNodeName5}
	node5, err = s.CreateOrUpdateNode(ctx, defaultClusterID, node5)
	re.NoError(err)

	// test list nodes
	nodes, err := s.ListNodes(ctx, defaultClusterID)
	re.NoError(err)

	re.Equal(node1.Name, nodes[0].Name)
	re.Equal(node1.CreateTime, nodes[0].CreateTime)
	re.Equal(node1.LastTouchTime, nodes[0].LastTouchTime)

	re.Equal(node2.Name, nodes[1].Name)
	re.Equal(node2.CreateTime, nodes[1].CreateTime)
	re.Equal(node2.LastTouchTime, nodes[1].LastTouchTime)

	re.Equal(node3.Name, nodes[2].Name)
	re.Equal(node3.CreateTime, nodes[2].CreateTime)
	re.Equal(node3.LastTouchTime, nodes[2].LastTouchTime)

	re.Equal(node4.Name, nodes[3].Name)
	re.Equal(node4.CreateTime, nodes[3].CreateTime)
	re.Equal(node4.LastTouchTime, nodes[3].LastTouchTime)

	re.Equal(node5.Name, nodes[4].Name)
	re.Equal(node5.CreateTime, nodes[4].CreateTime)
	re.Equal(node5.LastTouchTime, nodes[4].LastTouchTime)
}

func NewStorage(t *testing.T) Storage {
	cfg := etcdutil.NewTestSingleConfig()
	etcd, err := embed.StartEtcd(cfg)
	assert.NoError(t, err)

	<-etcd.Server.ReadyNotify()

	endpoint := cfg.LCUrls[0].String()
	client, err := clientv3.New(clientv3.Config{
		Endpoints: []string{endpoint},
	})
	assert.NoError(t, err)

	ops := Options{MaxScanLimit: 100, MinScanLimit: 10}

	return newEtcdStorage(client, defaultRootPath, ops)
}
