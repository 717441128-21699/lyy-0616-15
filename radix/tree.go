package radix

import (
	"fmt"
	"sort"
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
	typ      nodeType
	prefix   string
	children []*node
	handler  interface{}
	priority int
	param    string
	wildcard bool
	index    string
}

func newNode(prefix string, typ nodeType) *node {
	return &node{
		prefix:   prefix,
		typ:      typ,
		children: make([]*node, 0),
	}
}

type Tree struct {
	root *node
}

func NewTree() *Tree {
	return &Tree{
		root: &node{
			typ:      nodeRoot,
			prefix:   "",
			children: make([]*node, 0),
		},
	}
}

func (t *Tree) Add(method, path string, handler interface{}) {
	if path[0] != '/' {
		panic("path must start with '/'")
	}
	if path == "/" {
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
	isWildcard := false

	if len(seg) > 0 && seg[0] == ':' {
		typ = nodeParam
		parts := strings.SplitN(seg[1:], "/", 2)
		paramName = parts[0]
		if strings.HasSuffix(paramName, "*") {
			paramName = paramName[:len(paramName)-1]
			isWildcard = true
		}
	} else if len(seg) > 0 && seg[0] == '*' {
		typ = nodeWildcard
		paramName = seg[1:]
		isWildcard = true
	}

	for _, child := range parent.children {
		if child.typ == typ {
			if typ == nodeStatic && child.prefix == seg {
				return child
			}
			if typ == nodeParam {
				return child
			}
			if typ == nodeWildcard {
				return child
			}
		}
	}

	child := newNode(seg, typ)
	child.param = paramName
	child.wildcard = isWildcard
	parent.children = append(parent.children, child)
	parent.index += string(typChar(typ))
	sortChildren(parent)
	return child
}

func typChar(typ nodeType) byte {
	switch typ {
	case nodeStatic:
		return 's'
	case nodeParam:
		return 'p'
	case nodeWildcard:
		return 'w'
	default:
		return 'r'
	}
}

func sortChildren(n *node) {
	sort.SliceStable(n.children, func(i, j int) bool {
		return n.children[i].typ < n.children[j].typ
	})

	newIndex := ""
	for _, child := range n.children {
		newIndex += string(typChar(child.typ))
	}
	n.index = newIndex
}

type Result struct {
	Handler  interface{}
	Params   Params
	TSR      bool
	TSRPath  string
	Found    bool
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

	path = strings.TrimRight(path, "/")
	cleanPath := path
	if path[0] == '/' {
		path = path[1:]
	}

	segments := strings.Split(path, "/")
	params := make(Params, 0)
	current := t.root

	for i, seg := range segments {
		child := t.matchChild(current, seg)
		if child == nil {
			tsrPath := t.findTSR(current, cleanPath, segments, i)
			return Result{Found: false, TSR: tsrPath != "", TSRPath: tsrPath}
		}

		switch child.typ {
		case nodeParam:
			params = append(params, Param{Key: child.param, Value: seg})
		case nodeWildcard:
			rest := strings.Join(segments[i:], "/")
			params = append(params, Param{Key: child.param, Value: rest})
			current = child
			goto done
		}

		current = child
	}

done:
	if current.handler != nil {
		return Result{Handler: current.handler, Params: params, Found: true}
	}

	return Result{Found: false}
}

func (t *Tree) matchChild(parent *node, seg string) *node {
	for _, child := range parent.children {
		switch child.typ {
		case nodeStatic:
			if child.prefix == seg {
				return child
			}
		case nodeParam:
			return child
		case nodeWildcard:
			return child
		}
	}
	return nil
}

func (t *Tree) findTSR(current *node, originalPath string, segments []string, failedIdx int) string {
	if failedIdx == len(segments) {
		if len(current.children) == 0 {
			return originalPath + "/"
		}
		return ""
	}

	for _, child := range current.children {
		if child.typ == nodeStatic && child.prefix == segments[failedIdx] {
			remaining := strings.Join(segments[failedIdx+1:], "/")
			if remaining == "" {
				return originalPath + "/"
			}
			return ""
		}
	}
	return ""
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
	sb.WriteString(fmt.Sprintf("%s%s(%q) prio=%d%s\n", indent, typStr, n.prefix, n.priority, handlerStr))
	for _, child := range n.children {
		t.dumpNode(child, depth+1, sb)
	}
}
