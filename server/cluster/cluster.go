// Copyright 2022 CeresDB Project Authors. Licensed under Apache-2.0.

package cluster

import (
	"context"
	"sync"

	"github.com/CeresDB/ceresmeta/server/id"

	"github.com/CeresDB/ceresdbproto/pkg/clusterpb"
	"github.com/CeresDB/ceresmeta/server/storage"
	"github.com/pkg/errors"
)

type Cluster struct {
	clusterID uint32

	// RWMutex is used to project following fields
	lock         sync.RWMutex
	metaData     *metaData
	shardsCache  map[uint32]*Shard  // shard_id -> shard
	schemasCache map[string]*Schema // schema_name -> schema
	nodesCache   map[string]*Node   // node_name -> node

	storage     storage.Storage
	coordinator *coordinator
	alloc       id.Allocator
}

func NewCluster(cluster *clusterpb.Cluster, storage storage.Storage) *Cluster {
	alloc := id.NewAllocatorImpl(storage, "/", "alloc-id")
	return &Cluster{
		clusterID:    cluster.GetId(),
		storage:      storage,
		alloc:        alloc,
		metaData:     &metaData{cluster: cluster},
		shardsCache:  make(map[uint32]*Shard),
		schemasCache: make(map[string]*Schema),
		nodesCache:   make(map[string]*Node),
	}
}

func (c *Cluster) Name() string {
	return c.metaData.cluster.Name
}

func (c *Cluster) Load(ctx context.Context) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.coordinator = NewCoordinator(c)

	shards, shardIDs, err := c.loadClusterTopologyLocked(ctx)
	if err != nil {
		return errors.Wrap(err, "clusters Load")
	}

	shardTopologies, err := c.loadShardTopologyLocked(ctx, shardIDs)
	if err != nil {
		return errors.Wrap(err, "clusters Load")
	}

	schemas, err := c.loadSchemaLocked(ctx)
	if err != nil {
		return errors.Wrap(err, "clusters Load")
	}

	nodes, err := c.loadNodeLocked(ctx)
	if err != nil {
		return errors.Wrap(err, "clusters Load")
	}

	tables, err := c.loadTableLocked(ctx, schemas)
	if err != nil {
		return errors.Wrap(err, "clusters Load")
	}

	if err := c.updateCacheLocked(shards, shardTopologies, schemas, nodes, tables); err != nil {
		return errors.Wrap(err, "clusters Load")
	}
	return nil
}

// TODO: load node into nodesCache
func (c *Cluster) updateCacheLocked(
	shards map[uint32][]*clusterpb.Shard,
	shardTopologies map[uint32]*clusterpb.ShardTopology,
	schemasLoaded map[string]*clusterpb.Schema,
	nodesLoaded map[string]*clusterpb.Node,
	tablesLoaded map[string]map[uint64]*clusterpb.Table,
) error {
	for schemaName, tables := range tablesLoaded {
		for _, table := range tables {
			_, ok := c.schemasCache[schemaName]
			if ok {
				c.schemasCache[schemaName].tableMap[table.GetName()] = &Table{
					schema: schemasLoaded[schemaName],
					meta:   table,
				}
			} else {
				c.schemasCache[schemaName] = &Schema{
					meta: schemasLoaded[schemaName],
					tableMap: map[string]*Table{table.GetName(): {
						schema: schemasLoaded[schemaName],
						meta:   table,
					}},
				}
			}
		}
	}

	shardIDs := make(map[string][]uint32, len(nodesLoaded))
	for shardID, shards := range shards {
		for _, shard := range shards {
			shardIDs[shard.Node] = append(shardIDs[shard.Node], shardID)
		}
	}
	for nodeName, node := range nodesLoaded {
		c.nodesCache[nodeName] = &Node{meta: node, shardIDs: shardIDs[nodeName]}
	}

	for shardID, shardTopology := range shardTopologies {
		tables := make(map[uint64]*Table, len(shardTopology.TableIds))

		for _, tableID := range shardTopology.TableIds {
			for schemaName, tableMap := range tablesLoaded {
				table, ok := tableMap[tableID]
				if ok {
					tables[tableID] = &Table{
						schema: schemasLoaded[schemaName],
						meta:   table,
					}
				}
			}
		}
		// TODO: assert shardID
		// TODO: check shard not found by shardID
		shardMetaList := shards[shardID]
		var nodes []*clusterpb.Node
		for _, shardMeta := range shardMetaList {
			if node := c.nodesCache[shardMeta.Node]; node != nil {
				nodes = append(nodes, node.meta)
			}
		}
		c.shardsCache[shardID] = &Shard{
			meta:    shards[shardID],
			nodes:   nodes,
			tables:  tables,
			version: 0,
		}
	}

	for shardID, shardPbs := range shards {
		for _, shard := range shardPbs {
			if node := c.nodesCache[shard.GetNode()]; node != nil {
				if _, ok := c.nodesCache[shard.GetNode()]; ok {
					node.shardIDs = append(node.shardIDs, shardID)
				} else {
					node.shardIDs = []uint32{shardID}
				}
			}

		}
	}

	return nil
}

func (c *Cluster) updateSchemaCacheLocked(schemaPb *clusterpb.Schema) *Schema {
	schema := &Schema{meta: schemaPb, tableMap: make(map[string]*Table, 0)}
	c.schemasCache[schemaPb.GetName()] = schema
	return schema
}

func (c *Cluster) updateTableCacheLocked(schema *Schema, tablePb *clusterpb.Table) *Table {
	table := &Table{meta: tablePb, schema: schema.meta}
	schema.tableMap[tablePb.GetName()] = table
	c.shardsCache[tablePb.GetShardId()].tables[table.GetID()] = table
	return table
}

func (c *Cluster) GetTables(ctx context.Context, shardIDs []uint32, nodeName string) (map[uint32]*ShardTablesWithRole, error) {
	// TODO: refactor more fine-grained locks
	c.lock.RLock()
	defer c.lock.RUnlock()

	shardTables := make(map[uint32]*ShardTablesWithRole, len(shardIDs))
	for _, shardID := range shardIDs {
		shard, ok := c.shardsCache[shardID]
		if !ok {
			return nil, ErrShardNotFound.WithCausef("shardID:%d", shardID)
		}

		shardRole := clusterpb.ShardRole_FOLLOWER
		found := false
		for i, n := range shard.nodes {
			if nodeName == n.GetName() {
				found = true
				shardRole = shard.meta[i].ShardRole
				break
			}
		}
		if !found {
			return nil, ErrNodeNotFound.WithCausef("nodeName not found in current shard, shardID:%d, nodeName:%s", shardID, nodeName)
		}

		tables := make([]*Table, 0, len(shard.tables))
		for _, table := range shard.tables {
			tables = append(tables, table)
		}
		shardTables[shardID] = &ShardTablesWithRole{shardRole: shardRole, tables: tables, version: shard.version}
	}

	return shardTables, nil
}

func (c *Cluster) DropTable(ctx context.Context, schemaName, tableName string, tableID uint64) error {
	schema, exists := c.getSchemaLocked(schemaName)
	if exists {
		return ErrSchemaNotFound.WithCausef("schemaName:%s", schemaName)
	}

	if err := c.storage.DeleteTable(ctx, c.clusterID, schema.GetID(), tableName); err != nil {
		return errors.Wrapf(err, "clusters DropTable, clusterID:%d, schema:%v, tableName:%s",
			c.clusterID, schema, tableName)
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	schema.dropTableLocked(tableName)
	for _, shard := range c.shardsCache {
		shard.dropTableLocked(tableID)
	}
	return nil
}

func (c *Cluster) CreateSchema(ctx context.Context, schemaName string) (uint32, error) {
	// check if exists
	schema, exists := c.getSchemaLocked(schemaName)
	if exists {
		return schema.GetID(), nil
	}

	// alloc schema id
	schemaID, err := c.allocSchemaID(ctx)
	if err != nil {
		return 0, errors.Wrapf(err, "cluster AllocSchemaID, "+
			"schemaName:%s", schemaName)
	}

	// persist
	schemaPb := &clusterpb.Schema{Id: schemaID, Name: schemaName, ClusterId: c.clusterID}
	schemaPb, err = c.storage.CreateSchema(ctx, c.clusterID, schemaPb)
	if err != nil {
		return 0, errors.Wrap(err, "clusters CreateSchema")
	}

	// cache
	c.lock.Lock()
	defer c.lock.Unlock()
	_ = c.updateSchemaCacheLocked(schemaPb)
	return schemaID, nil
}

func (c *Cluster) CreateTable(ctx context.Context, schemaName string, shardID uint32,
	tableName string) (uint64, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Check provided schema if exists.
	schema, exists := c.getSchemaLocked(schemaName)
	if !exists {
		return 0, ErrSchemaNotFound.WithCausef("schemaName", schemaName)
	}

	// alloc table id
	tableID, err := c.allocTableID(ctx)
	if err != nil {
		return 0, errors.Wrapf(err, "cluster AllocTableID, "+
			"schemaName:%s, tableName:%s", schemaName, tableName)
	}

	// Save table in storage.
	tablePb := &clusterpb.Table{Id: tableID, Name: tableName, SchemaId: schema.GetID(), ShardId: shardID}
	tablePb, err = c.storage.CreateTable(ctx, c.clusterID, schema.GetID(), tablePb)
	if err != nil {
		return 0, errors.Wrap(err, "clusters CreateTable")
	}

	// Update tableCache in memory.
	_ = c.updateTableCacheLocked(schema, tablePb)
	return tableID, nil
}

func (c *Cluster) loadClusterTopologyLocked(ctx context.Context) (map[uint32][]*clusterpb.Shard, []uint32, error) {
	clusterTopology, err := c.storage.GetClusterTopology(ctx, c.clusterID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "clusters loadClusterTopologyLocked")
	}
	c.metaData.clusterTopology = clusterTopology

	if c.metaData.clusterTopology == nil {
		return nil, nil, ErrClusterTopologyNotFound.WithCausef("clusters:%v", c)
	}

	shardMap := make(map[uint32][]*clusterpb.Shard)
	for _, shard := range c.metaData.clusterTopology.ShardView {
		shardMap[shard.Id] = append(shardMap[shard.Id], shard)
	}

	shardIDs := make([]uint32, 0, len(shardMap))
	for id := range shardMap {
		shardIDs = append(shardIDs, id)
	}

	return shardMap, shardIDs, nil
}

func (c *Cluster) loadShardTopologyLocked(ctx context.Context, shardIDs []uint32) (map[uint32]*clusterpb.ShardTopology, error) {
	topologies, err := c.storage.ListShardTopologies(ctx, c.clusterID, shardIDs)
	if err != nil {
		return nil, errors.Wrap(err, "clusters loadShardTopologyLocked")
	}
	shardTopologyMap := make(map[uint32]*clusterpb.ShardTopology, len(shardIDs))
	for i, topology := range topologies {
		shardTopologyMap[shardIDs[i]] = topology
	}
	return shardTopologyMap, nil
}

func (c *Cluster) loadSchemaLocked(ctx context.Context) (map[string]*clusterpb.Schema, error) {
	schemas, err := c.storage.ListSchemas(ctx, c.clusterID)
	if err != nil {
		return nil, errors.Wrap(err, "clusters loadSchemaLocked")
	}
	schemaMap := make(map[string]*clusterpb.Schema, len(schemas))
	for _, schema := range schemas {
		schemaMap[schema.Name] = schema
	}
	return schemaMap, nil
}

func (c *Cluster) loadNodeLocked(ctx context.Context) (map[string]*clusterpb.Node, error) {
	nodes, err := c.storage.ListNodes(ctx, c.clusterID)
	if err != nil {
		return nil, errors.Wrap(err, "clusters loadNodeLocked")
	}
	nodeMap := make(map[string]*clusterpb.Node, len(nodes))
	for _, node := range nodes {
		nodeMap[node.Name] = node
	}
	return nodeMap, nil
}

func (c *Cluster) loadTableLocked(ctx context.Context, schemas map[string]*clusterpb.Schema) (map[string]map[uint64]*clusterpb.Table, error) {
	tables := make(map[string]map[uint64]*clusterpb.Table)
	for _, schema := range schemas {
		tablePbs, err := c.storage.ListTables(ctx, c.clusterID, schema.Id)
		if err != nil {
			return nil, errors.Wrap(err, "clusters loadTableLocked")
		}
		for _, table := range tablePbs {
			if t, ok := tables[schema.GetName()]; ok {
				t[table.GetId()] = table
			} else {
				tables[schema.GetName()] = map[uint64]*clusterpb.Table{table.GetId(): table}
			}
		}
	}
	return tables, nil
}

func (c *Cluster) getSchemaLocked(schemaName string) (*Schema, bool) {
	schema, ok := c.schemasCache[schemaName]
	return schema, ok
}

func (c *Cluster) GetTable(ctx context.Context, schemaName, tableName string) (*Table, bool, error) {
	c.lock.RLock()
	schema, ok := c.schemasCache[schemaName]
	if !ok {
		c.lock.RUnlock()
		return nil, false, ErrSchemaNotFound.WithCausef("schemaName", schemaName)
	}

	table, exists := schema.getTable(tableName)
	if exists {
		c.lock.RUnlock()
		return table, true, nil
	}
	c.lock.RUnlock()

	// Search table in storage.
	tablePb, exists, err := c.storage.GetTable(ctx, c.clusterID, schema.GetID(), tableName)
	if err != nil {
		return nil, false, errors.Wrap(err, "clusters GetTable")
	}
	if exists {
		c.lock.Lock()
		defer c.lock.Unlock()
		table = c.updateTableCacheLocked(schema, tablePb)
		return table, true, nil
	}

	return nil, false, nil
}

func (c *Cluster) GetShardIDs(nodeName string) ([]uint32, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	node, ok := c.nodesCache[nodeName]
	if !ok {
		return nil, ErrNodeNotFound.WithCausef("clusters GetShardIDs, nodeName:%s", nodeName)
	}
	return node.shardIDs, nil
}

func (c *Cluster) RegisterNode(ctx context.Context, nodeName string, lease uint32) error {
	nodePb := &clusterpb.Node{NodeStats: &clusterpb.NodeStats{Lease: lease}, Name: nodeName}
	nodePb1, err := c.storage.CreateOrUpdateNode(ctx, c.clusterID, nodePb)
	if err != nil {
		return errors.Wrapf(err, "clusters manager RegisterNode, nodeName:%s", nodeName)
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	node, ok := c.nodesCache[nodeName]
	if ok {
		node.meta = nodePb1
	} else {
		c.nodesCache[nodeName] = &Node{meta: nodePb1}
	}
	return nil
}

func (c *Cluster) allocSchemaID(ctx context.Context) (uint32, error) {
	ID, err := c.alloc.Alloc(ctx)
	if err != nil {
		return 0, errors.Wrapf(err, "alloc schema id failed")
	}
	return uint32(ID), nil
}

func (c *Cluster) allocTableID(ctx context.Context) (uint64, error) {
	ID, err := c.alloc.Alloc(ctx)
	if err != nil {
		return 0, errors.Wrapf(err, "alloc table id failed")
	}
	return ID, nil
}

type metaData struct {
	cluster         *clusterpb.Cluster
	clusterTopology *clusterpb.ClusterTopology
}