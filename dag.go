package internal

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"sync"
)

type id = uuid.UUID
type idSet = map[id]bool

type Vertex interface {
	String() string
}

var space *id = nil

func getSpace() id {
	if space == nil {
		random, _ := uuid.NewRandom()
		space = &random
	}
	return *space
}

func idFormName(name string) id {
	return uuid.NewMD5(getSpace(), []byte(name))
}

// DAG type implements a Directed Acyclic Graph data structure.
type DAG struct {
	vertices     map[id]*Vertex
	name2id      map[string]id
	id2name      map[id]string
	muVertices   sync.Mutex
	inboundEdge  map[id]idSet
	outboundEdge map[id]idSet
	muEdges      sync.Mutex
}

// Creates a new Directed Acyclic Graph or DAG.
func NewDAG() *DAG {
	d := &DAG{
		vertices:     make(map[id]*Vertex),
		inboundEdge:  make(map[id]idSet),
		outboundEdge: make(map[id]idSet),
		name2id:      make(map[string]id),
		id2name:      make(map[id]string),
	}
	return d
}

// Add a vertex.
func (d *DAG) AddVertex(name string, v *Vertex) error {
	if _, ok := d.name2id[name]; ok {
		return errors.New(fmt.Sprintf("duplicate entry for name %s", name))
	}
	d.muVertices.Lock()
	id := idFormName(name)
	d.name2id[name] = id
	d.id2name[id] = name
	d.vertices[id] = v
	d.muVertices.Unlock()
	return nil
}

// Delete a vertex including all inbound and outbound edges.
func (d *DAG) DeleteVertex(name string) {
	if id, ok := d.name2id[name]; ok {
		d.muEdges.Lock()
		delete(d.inboundEdge, id)
		delete(d.outboundEdge, id)
		d.muEdges.Unlock()
		d.muVertices.Lock()
		delete(d.vertices, id)
		delete(d.id2name, id)
		delete(d.name2id, name)
		d.muVertices.Unlock()
	}
}

// Add an edge, iff both vertices exist.
func (d *DAG) AddEdge(srcName string, dstName string) error {

	// sanity checking
	if srcName == dstName {
		return errors.New(fmt.Sprintf("src name (%s) and dst name (%s) must be different", srcName, dstName))
	}
	srcId, srcExists := d.name2id[srcName]
	if !srcExists {
		return errors.New(fmt.Sprintf("src name %s does not exist", srcName))
	}
	dstId, dstExists := d.name2id[dstName]
	if !dstExists {
		return errors.New(fmt.Sprintf("dst name %s does not exist", dstName))
	}

	// test / compute edge nodes
	outbound, outboundExists := d.outboundEdge[srcId]
	inbound, inboundExists := d.inboundEdge[dstId]

	d.muEdges.Lock()

	// add outbound
	if !outboundExists {
		newSet := make(idSet)
		d.outboundEdge[srcId] = newSet
		outbound = newSet
	}
	outbound[dstId] = true

	// add inbound
	if !inboundExists {
		newSet := make(idSet)
		d.inboundEdge[dstId] = newSet
		inbound = newSet
	}
	inbound[dstId] = true

	d.muEdges.Unlock()
	return nil
}

// Delete an edge, iff such exists.
func (d *DAG) DeleteEdge(srcName string, dstName string) error {
	// sanity checking
	if srcName == dstName {
		return errors.New(fmt.Sprintf("src name (%s) and dst name (%s) must be different", srcName, dstName))
	}
	srcId, srcExists := d.name2id[srcName]
	if !srcExists {
		return errors.New(fmt.Sprintf("src name %s does not exist", srcName))
	}
	dstId, dstExists := d.name2id[dstName]
	if !dstExists {
		return errors.New(fmt.Sprintf("dst name %s does not exist", dstName))
	}

	// test / compute edge nodes
	_, outboundExists := d.outboundEdge[srcId][dstId]
	_, inboundExists := d.inboundEdge[dstId][srcId]

	if inboundExists || outboundExists {
		d.muEdges.Lock()

		// delete outbound
		if outboundExists {
			delete(d.inboundEdge[dstId], dstId)
		}

		// delete inbound
		if inboundExists {
			delete(d.outboundEdge[srcId], dstId)
		}
		d.muEdges.Unlock()
	}

	return nil
}

// Return the vertex with the given id.
func (d *DAG) GetVertex(name string) (*Vertex, error) {
	id, exists := d.name2id[name]
	if !exists {
		return nil, errors.New(fmt.Sprintf("name %s does not exist", name))
	}
	return d.vertices[id], nil
}

// Order return the total number of vertices.
func (d *DAG) Order() int {
	return len(d.vertices)
}

// Return the total number of edges.
func (d *DAG) Size() int {
	count := 0
	for _, value := range d.outboundEdge {
		count += len(value)
	}
	return count
}

// Return all vertices without children.
func (d *DAG) Leafs() []*Vertex {
	var leafs []*Vertex
	for id := range d.vertices {
		dstIds, ok := d.outboundEdge[id]
		if !ok || len(dstIds) == 0 {
			leafs = append(leafs, d.vertices[id])
		}
	}
	return leafs
}

// Return all children for the vertex with the given id.
func (d *DAG) Children(name string) ([]*Vertex, error) {
	id, exists := d.name2id[name]
	if !exists {
		return nil, errors.New(fmt.Sprintf("name %s does not exist", name))
	}
	var children []*Vertex
	if dstIds, ok := d.outboundEdge[id]; ok {
		for id := range dstIds {
			children = append(children, d.vertices[id])
		}
		return children, nil
	}
	return nil, nil
}

func (d *DAG) String() string {
	result := fmt.Sprintf("DAG Vertices: %d - Edges: %d\n", d.Order(), d.Size())
	result += fmt.Sprintf("Vertices:\n")
	for _, v := range d.vertices {
		result += fmt.Sprintf("  %s\n", (*v).String())
	}
	result += fmt.Sprintf("Edges:\n")
	for srcId, dsts := range d.outboundEdge {
		for dstId := range dsts {
			result += fmt.Sprintf("  %s -> %s\n", (*d.vertices[srcId]).String(), (*d.vertices[dstId]).String())
		}
	}
	return result
}