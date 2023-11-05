package ioc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrValueNotFound = errors.New("ioc: value not found")

	contextKey = struct{ name string }{"ioc"}
	tagName    = "ioc"
)

type Container struct {
	parent    *Container
	factories map[reflect.Type]*binding
	instances map[reflect.Type]reflect.Value
	names     map[string]reflect.Type
}

func New() *Container {
	return &Container{}
}

func (c *Container) Fork() *Container {
	return &Container{parent: c}
}

func (c *Container) Bind(value any) {
	c.NamedBind("", value)
}

func (c *Container) NamedBind(name string, value any) {
	rt := reflect.TypeOf(value)
	c.setName(name, rt)
	c.setInstance(rt, reflect.ValueOf(value))
}

func (c *Container) Factory(factory any, shared ...bool) error {
	return c.NamedFactory("", factory)
}

func (c *Container) NamedFactory(name string, factory any, shared ...bool) error {
	b, err := newBinding(factory, shared...)
	if err != nil {
		return err
	}
	if c.factories == nil {
		c.factories = make(map[reflect.Type]*binding)
	}
	c.factories[b.typ] = b
	c.setName(name, b.typ)
	return nil
}

func (c *Container) setInstance(rt reflect.Type, rv reflect.Value) {
	if c.instances == nil {
		c.instances = make(map[reflect.Type]reflect.Value)
	}
	c.instances[rt] = rv
}

func (c *Container) setName(name string, typ reflect.Type) {
	if name != "" {
		if c.names == nil {
			c.names = make(map[string]reflect.Type)
		}
		c.names[name] = typ
	}
}

func (c *Container) Get(t reflect.Type) (reflect.Value, error) {
	if t == nil {
		return reflect.Value{}, ErrValueNotFound
	}
	return c.get("", t)
}

func (c *Container) NamedGet(name string, t reflect.Type) (reflect.Value, error) {
	if t == nil {
		return reflect.Value{}, ErrValueNotFound
	}
	if name != "" {
		if rt, ok := c.names[name]; ok && rt != t {
			return c.get(name, rt, t)
		}
	}
	return c.get(name, t)
}

func (c *Container) get(name string, ts ...reflect.Type) (reflect.Value, error) {
	for _, t := range ts {
		if val, instanced := c.instances[t]; instanced && val.IsValid() {
			return val, nil
		}
		if binder, bound := c.factories[t]; bound {
			val, err := binder.make(c)
			if err != nil {
				// TODO(hupeh): 更加友好的错误信息
				return reflect.Value{}, err
			}
			if val.IsValid() {
				return val, nil
			}
		}
		for n, rt := range c.names {
			if n == name || rt == t {
				// fixme: 是否可以比较
				// t.Comparable()
				continue
			}
			switch {
			case t.Kind() == reflect.Interface && rt.Implements(t),
				rt.AssignableTo(t):
				val, err := c.Get(rt)
				if err != nil {
					return val, nil
				}
			}
		}
	}
	if c.parent != nil {
		return c.NamedGet(name, ts[len(ts)-1])
	}
	return reflect.Value{}, ErrValueNotFound
}

// Resolve 完成的注入
//
// 我们的结构体中，可以通过指定 `ioc` tag 使用指定的注入。
func (c *Container) Resolve(i any) error {
	v := reflect.ValueOf(i)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return errors.New("ioc: must given a struct")
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		tag, omitempty, inject := parseTag(t.Field(i))
		if !f.CanSet() {
			if inject && !omitempty {
				return fmt.Errorf("ioc: cannot make %v field", t.Field(i).Name)
			}
			continue
		}
		ft := f.Type()
		fv, err := c.NamedGet(tag, ft)
		if err != nil {
			if omitempty {
				continue
			}
			// TODO(hupeh): 更加友好的错误提示
			return err
		}
		if !fv.IsValid() {
			return fmt.Errorf("ioc: value not found for type %v", ft)
		}
		f.Set(fv)
	}
	return nil
}

func (c *Container) Invoke(fn any) ([]reflect.Value, error) {
	rt := reflect.TypeOf(fn)
	if rt.Kind() != reflect.Func {
		return nil, errors.New("ioc: Out of non-func type " + rt.String())
	}
	return c.invoke(rt, reflect.ValueOf(fn))
}

func (c *Container) invoke(rt reflect.Type, rv reflect.Value) ([]reflect.Value, error) {
	var in = make([]reflect.Value, rt.NumIn())
	for i := 0; i < rt.NumIn(); i++ {
		argType := rt.In(i)
		val, err := c.Get(argType)
		if err != nil {
			return nil, err
		}
		if !val.IsValid() {
			return nil, fmt.Errorf("ioc: value not found for type %v", argType)
		}
		in[i] = val
	}
	return rv.Call(in), nil
}

func (c *Container) NewContext(parentCtx ...context.Context) context.Context {
	for _, ctx := range parentCtx {
		if ctx != nil {
			return context.WithValue(ctx, contextKey, c)
		}
	}
	return context.WithValue(context.Background(), contextKey, c)
}
