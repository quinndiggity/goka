package goka

import (
	"errors"
	"fmt"
)

//type Stream string
//type Table string
//type Group string

type GroupGraph struct {
	group         string
	inputTables   []Edge
	crossTables   []Edge
	inputStreams  []Edge
	outputStreams []Edge
	loopStream    []Edge
	groupTable    []Edge

	codecs    map[string]Codec
	callbacks map[string]ConsumeCallback
}

func (gg *GroupGraph) Group() string {
	return gg.group
}

func (gg *GroupGraph) InputStreams() Edges {
	return append(gg.inputStreams, gg.loopStream...)
}

func (gg *GroupGraph) JointTables() Edges {
	return gg.inputTables
}

func (gg *GroupGraph) LookupTables() Edges {
	return gg.crossTables
}

func (gg *GroupGraph) getLoopStream() Edge {
	// only 1 loop stream is valid
	if len(gg.loopStream) > 0 {
		return gg.loopStream[0]
	}
	return nil
}

func (gg *GroupGraph) GroupTable() Edge {
	// only 1 group table is valid
	if len(gg.groupTable) > 0 {
		return gg.groupTable[0]
	}
	return nil
}

func (gg *GroupGraph) OutputStreams() Edges {
	return append(gg.outputStreams, gg.loopStream...)
}

// inputs returns all input topics (tables and streams)
func (gg *GroupGraph) inputs() Edges {
	return append(append(gg.inputStreams, gg.inputTables...), gg.crossTables...)
}

func (gg *GroupGraph) codec(topic string) Codec {
	return gg.codecs[topic]
}

func (gg *GroupGraph) callback(topic string) ConsumeCallback {
	return gg.callbacks[topic]
}

func (gg *GroupGraph) joint(topic string) bool {
	for _, t := range gg.inputTables {
		if t.Topic() == topic {
			return true
		}
	}
	return false
}

func DefineGroup(group string, edges ...Edge) *GroupGraph {
	gg := GroupGraph{group: group,
		codecs:    make(map[string]Codec),
		callbacks: make(map[string]ConsumeCallback),
	}

	for _, e := range edges {
		switch e := e.(type) {
		case *inputStream:
			gg.codecs[e.Topic()] = e.Codec()
			gg.callbacks[e.Topic()] = e.cb
			gg.inputStreams = append(gg.inputStreams, e)
		case *loopStream:
			e.setGroup(group)
			gg.codecs[e.Topic()] = e.Codec()
			gg.callbacks[e.Topic()] = e.cb
			gg.loopStream = append(gg.loopStream, e)
		case *outputStream:
			gg.codecs[e.Topic()] = e.Codec()
			gg.outputStreams = append(gg.outputStreams, e)
		case *inputTable:
			gg.codecs[e.Topic()] = e.Codec()
			gg.inputTables = append(gg.inputTables, e)
		case *crossTable:
			gg.codecs[e.Topic()] = e.Codec()
			gg.crossTables = append(gg.crossTables, e)
		case *groupTable:
			e.setGroup(group)
			gg.codecs[e.Topic()] = e.Codec()
			gg.groupTable = append(gg.groupTable, e)
		}
	}
	return &gg
}

func (gg *GroupGraph) Validate() error {
	if len(gg.loopStream) > 1 {
		return errors.New("more than one loop stream in group graph")
	}
	if len(gg.groupTable) > 1 {
		return errors.New("more than one group table in group graph")
	}
	if len(gg.inputStreams) == 0 {
		return errors.New("no input stream in group graph")
	}
	for _, t := range append(gg.outputStreams,
		append(gg.inputStreams, append(gg.inputTables, gg.crossTables...)...)...) {
		if t.Topic() == loopName(gg.group) {
			return errors.New("should not directly use loop stream")
		}
		if t.Topic() == GroupTable(gg.group) {
			return errors.New("should not directly use group table")
		}
	}
	return nil
}

type Edge interface {
	String() string
	Topic() string
	Codec() Codec
}

type Edges []Edge

func (e Edges) Topics() []string {
	var t []string
	for _, i := range e {
		t = append(t, i.Topic())
	}
	return t
}

type topicDef struct {
	name  string
	codec Codec
}

func (t *topicDef) Topic() string {
	return t.name
}

func (t *topicDef) String() string {
	return fmt.Sprintf("%s/%T", t.name, t.codec)
}

func (t *topicDef) Codec() Codec {
	return t.codec
}

type inputStream struct {
	*topicDef
	cb ConsumeCallback
}

// Stream returns a subscription for a co-partitioned topic. The processor
// subscribing for a stream topic will start reading from the newest offset of
// the partition.
func Input(stream string, c Codec, cb ConsumeCallback) Edge {
	return &inputStream{&topicDef{stream, c}, cb}
}

type loopStream inputStream

// Loop defines a consume callback on the loop topic
func Loop(c Codec, cb ConsumeCallback) Edge {
	return &loopStream{&topicDef{codec: c}, cb}
}

func (s *loopStream) setGroup(group string) {
	s.topicDef.name = loopName(group)
}

type inputTable struct {
	*topicDef
}

// Table is one or more co-partitioned, log-compacted topic. The processor
// subscribing for a table topic will start reading from the oldest offset
// of the partition.
func Join(table string, c Codec) Edge {
	return &inputTable{&topicDef{table, c}}
}

type crossTable struct {
	*topicDef
}

func Lookup(table string, c Codec) Edge {
	return &crossTable{&topicDef{table, c}}
}

type groupTable struct {
	*topicDef
}

func Persist(c Codec) Edge {
	return &groupTable{&topicDef{codec: c}}
}

func (t *groupTable) setGroup(group string) {
	t.topicDef.name = GroupTable(group)
}

type outputStream struct {
	*topicDef
}

func Output(stream string, c Codec) Edge {
	return &outputStream{&topicDef{stream, c}}
}

// GroupTable returns the name of the group table of group.
func GroupTable(group string) string {
	return group + "-state"
}

// loopName returns the name of the loop topic of group.
func loopName(group string) string {
	return group + "-loop"
}
