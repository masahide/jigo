package jigo

import (
	"fmt"
	"reflect"
	"strings"
)

// A context represents an environment passed in by a user to a template.  Certain
// tags can create temporary contexts (for, macro, etc), which get created at eval
// time.
type Context struct {
	ctx   interface{}
	kind  reflect.Kind
	value reflect.Value
}

// Contexts can be structs or maps, or pointers to these types, but no other type.
func NewContext(i interface{}) (*Context, error) {
	// save the original value, though we likely won't use it
	var v reflect.Value
	c := &Context{ctx: i}
	// indirect v
	for v = reflect.ValueOf(i); v.Kind() == reflect.Ptr; v = reflect.Indirect(v) {
	}
	c.kind = v.Kind()
	c.value = v
	if c.kind != reflect.Map && c.kind != reflect.Struct {
		return c, fmt.Errorf("Context must be a struct or map, not %s")
	}
	return c, nil
}

// lookup finds a single name in a single context.  If no name is found, then
// an empty Value is returned and ok is False.
func (c Context) lookup(name string) (v reflect.Value, ok bool) {
	pname, cname, err := hasChild(name)
	switch c.kind {
	case reflect.Map:
		if err != nil {
			return v, false
		}
		v := c.value.MapIndex(reflect.ValueOf(pname))
		if cname == "" {
			return v, v.IsValid()
		}
		switch t := v.Interface().(type) {
		case Context:
			return t.lookup(cname)
		}
		return v, false
	case reflect.Struct:
		// FIXME: reflectx fieldmaps will be much faster but a fair bit more code.
		// We should use them eventually.
		v := c.value.FieldByName(pname)
		if cname == "" {
			return v, v.IsValid()
		}
		switch t := v.Interface().(type) {
		case Context:
			return t.lookup(cname)
		}
		return v, false
	default:
		return v, false
	}
}

func hasChild(name string) (parent, child string, err error) {
	i := strings.IndexAny(name, ".[")
	if i == 0 {
		switch name[i] {
		case '[':
			name = name[1:]
			i := strings.Index(name, "]")
			if i == -1 {
				return "", "", fmt.Errorf("']' is not found. [%s]", name)
			}
			parent = name[:i]
			child = name[i+1:]
			if (parent[0] == '\'' && parent[len(parent)-1] == '\'') || (parent[0] == '"' && parent[len(parent)-1] == '"') {
				parent = parent[1 : len(parent)-1]
			}
			return
		case '.':
			name = name[1:]
			i = strings.IndexAny(name, ".[")
		}
	}
	if i == -1 {
		parent = name
		return
	}
	parent = name[:i]
	child = name[i:]
	return
}

// A stack of contexts.  Lookup failures go up the stack until there's a success
// or a final failure.  This is the way you get nested scopes.
type contextStack []*Context

func NewContextStack(i interface{}) contextStack {
	c := make(contextStack, 0, 4)
	ctx, err := NewContext(i)
	if err != nil {
		panic(err)
	}
	c.push(ctx)
	return c
}

func (c *contextStack) push(ctx *Context) {
	*c = append(*c, ctx)
}

func (c *contextStack) pop() (ctx *Context) {
	ctx = (*c)[len(*c)-1]
	*c = (*c)[:len(*c)-1]
	return ctx
}

// lookup finds a name in the context stack.  If no name is found, then an undefined
// sentinel is returned.
func (c contextStack) lookup(name string) (v reflect.Value, ok bool) {
	var ctx *Context
	for i := len(c) - 1; i >= 0; i-- {
		ctx = c[i]
		v, ok = ctx.lookup(name)
		if ok {
			return v, ok
		}
	}
	return v, ok
}
