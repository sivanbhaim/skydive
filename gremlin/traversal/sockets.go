/*
 * Copyright (C) 2018 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package traversal

import (
	"encoding/json"

	"github.com/mitchellh/mapstructure"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/traversal"
	"github.com/skydive-project/skydive/topology/probes/socketinfo"
)

// SocketsTraversalExtension describes a new extension to enhance the topology
type SocketsTraversalExtension struct {
	SocketsToken traversal.Token
}

// SocketsGremlinTraversalStep describes the Sockets gremlin traversal step
type SocketsGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
}

// NewMetricsTraversalExtension returns a new graph traversal extension
func NewSocketsTraversalExtension() *SocketsTraversalExtension {
	return &SocketsTraversalExtension{
		SocketsToken: traversalSocketsToken,
	}
}

// ScanIdent returns an associated graph token
func (e *SocketsTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "SOCKETS":
		return e.SocketsToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parse connections step
func (e *SocketsTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.SocketsToken:
		return &SocketsGremlinTraversalStep{context: p}, nil
	}
	return nil, nil
}

// Exec executes the metrics step
func (s *SocketsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		return Sockets(tv), nil
	case *FlowTraversalStep:
		return tv.Sockets(), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce flow step
func (s *SocketsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) traversal.GremlinTraversalStep {
	return next
}

// Context sockets step
func (s *SocketsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.context
}

// SocketsTraversalStep connections step
type SocketsTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	sockets        map[string][]*socketinfo.ConnectionInfo
	error          error
}

// PropertyValues returns a flow field value
func (s *SocketsTraversalStep) PropertyValues(keys ...interface{}) *traversal.GraphTraversalValue {
	if s.error != nil {
		return traversal.NewGraphTraversalValue(s.GraphTraversal, nil, s.error)
	}

	key := keys[0].(string)
	var values []interface{}
	for _, sockets := range s.sockets {
		for _, socket := range sockets {
			v, err := socket.GetField(key)
			if err != nil {
				return traversal.NewGraphTraversalValue(s.GraphTraversal, nil, common.ErrFieldNotFound)
			}
			values = append(values, v)
		}
	}

	return traversal.NewGraphTraversalValue(s.GraphTraversal, values, nil)
}

// Values returns list of socket informations
func (s *SocketsTraversalStep) Values() []interface{} {
	if len(s.sockets) == 0 {
		return []interface{}{}
	}
	return []interface{}{s.sockets}
}

// MarshalJSON serialize in JSON
func (s *SocketsTraversalStep) MarshalJSON() ([]byte, error) {
	values := s.Values()
	s.GraphTraversal.RLock()
	defer s.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

// Error returns traversal error
func (s *SocketsTraversalStep) Error() error {
	return s.error
}

func getSockets(n *graph.Node) (sockets []*socketinfo.ConnectionInfo) {
	if socks, err := n.GetField("Sockets"); err == nil {
		for _, socket := range socks.([]interface{}) {
			var conn socketinfo.ConnectionInfo
			if err := mapstructure.WeakDecode(socket, &conn); err == nil {
				sockets = append(sockets, &conn)
			}
		}
	}
	return
}

// NewSocketIndexer returns a new socket graph indexer
func NewSocketIndexer(g *graph.Graph) *graph.GraphIndexer {
	hashNode := func(n *graph.Node) map[string]interface{} {
		sockets := getSockets(n)
		kv := make(map[string]interface{}, len(sockets))
		for _, socket := range sockets {
			kv[socket.Hash()] = socket
		}
		return kv
	}

	graphIndexer := graph.NewGraphIndexer(g, hashNode, true)
	socketFilter := graph.NewGraphElementFilter(filters.NewNotNullFilter("Sockets"))
	for _, node := range g.GetNodes(socketFilter) {
		graphIndexer.OnNodeAdded(node)
	}
	return graphIndexer
}
