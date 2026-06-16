package radix

import (
	"fmt"
	"strings"
)

type nodeType uint8

const (
	nodeStatic   nodeType = iota
	nodeParam
	nodeWildcard
	nodeRoot
)

type node struct {
	typ           nodeType
	segment       string
	children      []*node
	staticIndex   map[string]*node
	paramChild    *node
	wildcardChild *node
	handler       interface{}
	priority      int
	paramName     string
}

func newNode(segment string, typ nodeType) *node {
	n := &node{
		segment:     segment,
		typ:       typ,
		children:  make([]*node, 0, 4),
		staticIndex: make(map[string]*node),
	}
	return n
}

type Tree struct {
	root *node
}

func NewTree() *Tree {
	return &Tree{
		root: &node{
			typ:         nodeRoot,
			segment:     "",
			children:    make([]*node, 0, 8),
			staticIndex: make(map[string]*node),
		},
	}
}

func (t *Tree) Add(method, path string, handler interface{}) {
	if len(path) == 0 || path[0] != '/' {
		panic("path must start with '/'")
	}
	if path == "/" {
		if t.root.handler != nil {
			panic("route conflict: 'GET /' already registered")
		}
		t.root.handler = handler
		t.root.priority++
		return
	}

	path = strings.TrimRight(path, "/")
	if path[0] == '/' {
		path = path[1:]
	}

	segments := strings.Split(path, "/")
	current := t.root
	current.priority++

	for i, seg := range segments {
		child := t.findOrAddChild(current, seg)
		child.priority++
		current = child

		if i == len(segments)-1 {
			if child.handler != nil {
				panic(fmt.Sprintf("route conflict: '%s %s' already registered", method, "/"+strings.Join(segments, "/")))
			}
			child.handler = handler
		}
	}
}

func (t *Tree) findOrAddChild(parent *node, seg string) *node {
	typ := nodeStatic
	paramName := ""

	if len(seg) > 0 && seg[0] == ':' {
		typ = nodeParam
		paramName = seg[1:]
	} else if len(seg) > 0 && seg[0] == '*' {
		typ = nodeWildcard
		paramName = seg[1:]
	}

	switch typ {
	case nodeStatic:
		if child, ok := parent.staticIndex[seg]; ok {
			return child
		}
		child := newNode(seg, typ)
		parent.staticIndex[seg] = child
		parent.children = append(parent.children, child)
		t.sortChildren(parent)
		return child

	case nodeParam:
		if parent.paramChild != nil {
			return parent.paramChild
		}
		child := newNode(seg, typ)
		child.paramName = paramName
		parent.paramChild = child
		parent.children = append(parent.children, child)
		t.sortChildren(parent)
		return child

	case nodeWildcard:
		if parent.wildcardChild != nil {
			return parent.wildcardChild
		}
		child := newNode(seg, typ)
		child.paramName = paramName
		parent.wildcardChild = child
		parent.children = append(parent.children, child)
		t.sortChildren(parent)
		return child
	}
	return nil
}

func (t *Tree) sortChildren(n *node) {
	staticNodes := make([]*node, 0)
	paramNode := n.paramChild
	wildcardNode := n.wildcardChild

	for _, child := range n.children {
		if child.typ == nodeStatic {
			staticNodes = append(staticNodes, child)
		}
	}

	sorted := make([]*node, 0, len(n.children))
	sorted = append(sorted, staticNodes...)
	if paramNode != nil {
		sorted = append(sorted, paramNode)
	}
	if wildcardNode != nil {
		sorted = append(sorted, wildcardNode)
	}
	n.children = sorted
}

type Result struct {
	Handler interface{}
	Params  Params
	TSR     bool
	TSRPath string
	Found   bool
}

type Param struct {
	Key   string
	Value string
}

type Params []Param

func (ps Params) ByName(name string) string {
	for _, p := range ps {
		if p.Key == name {
			return p.Value
		}
	}
	return ""
}

func (t *Tree) Get(method, path string) Result {
	if path == "/" {
		if t.root.handler != nil {
			return Result{Handler: t.root.handler, Found: true}
		}
		return Result{Found: false}
	}

	cleanPath := path
	path = strings.TrimRight(path, "/")
	if path[0] == '/' {
		path = path[1:]
	}

	segments := strings.Split(path, "/")
	params := make(Params, 0, 4)

	result := t.matchRecursive(t.root, segments, 0, params)

	if !result.Found {
		result.TSR = t.checkTSR(t.root, segments)
		if result.TSR {
			result.TSRPath = cleanPath + "/"
		}
	}

	return result
}

func (t *Tree) matchRecursive(current *node, segments []string, idx int, params Params) Result {
	if idx == len(segments) {
		if current.handler != nil {
			return Result{Handler: current.handler, Params: params, Found: true}
		}
		return Result{Found: false}
	}

	seg := segments[idx]

	if staticChild, ok := current.staticIndex[seg]; ok {
		result := t.matchRecursive(staticChild, segments, idx+1, params)
		if result.Found {
			return result
		}
	}

	if current.paramChild != nil {
		newParams := append(params, Param{Key: current.paramChild.paramName, Value: seg})
		result := t.matchRecursive(current.paramChild, segments, idx+1, newParams)
		if result.Found {
			return result
		}
	}

	if current.wildcardChild != nil {
		rest := strings.Join(segments[idx:], "/")
		newParams := append(params, Param{Key: current.wildcardChild.paramName, Value: rest})
		return Result{Handler: current.wildcardChild.handler, Params: newParams, Found: current.wildcardChild.handler != nil}
	}

	return Result{Found: false}
}

func (t *Tree) checkTSR(current *node, segments []string) bool {
	if len(segments) == 0 {
		return current.handler != nil
	}

	seg := segments[0]
	rest := segments[1:]

	if staticChild, ok := current.staticIndex[seg]; ok {
		if len(rest) == 0 {
			return true
		}
		return t.checkTSR(staticChild, rest)
	}

	return false
}

func (t *Tree) Dump() string {
	var sb strings.Builder
	t.dumpNode(t.root, 0, &sb)
	return sb.String()
}

func (t *Tree) dumpNode(n *node, depth int, sb *strings.Builder) {
	indent := strings.Repeat("  ", depth)
	typStr := ""
	switch n.typ {
	case nodeStatic:
		typStr = "STATIC"
	case nodeParam:
		typStr = "PARAM"
	case nodeWildcard:
		typStr = "WILDCARD"
	case nodeRoot:
		typStr = "ROOT"
	}
	handlerStr := ""
	if n.handler != nil {
		handlerStr = " [HANDLER]"
	}
	sb.WriteString(fmt.Sprintf("%s%s(%q) prio=%d static=%d%s\n", indent, typStr, n.segment, n.priority, len(n.staticIndex), handlerStr))
	for _, child := range n.children {
		t.dumpNode(child, depth+1, sb)
	}
}
