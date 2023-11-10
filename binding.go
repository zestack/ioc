package ioc

import (
	"errors"
	"reflect"
)

var (
	errorType = reflect.TypeOf(error(nil))

	errNotFactory        = errors.New("ioc: the factory must be a function")
	errInvalidFactory    = errors.New("ioc: factory function signature is invalid - it must return abstract, or abstract and error")
	errCircularReference = errors.New("ioc: factory function signature is invalid - depends on abstract it returns")
)

type binding struct {
	name    string
	typ     reflect.Type
	factory reflect.Value
	shared  bool
}

func newBinding(name string, factory any, shared ...bool) (*binding, error) {
	rv := reflect.ValueOf(factory)
	rt := rv.Type()
	if rt.Kind() != reflect.Func {
		return nil, errNotFactory
	}
	switch returnCount := rt.NumOut(); returnCount {
	case 1:
		// 只有一个返回值
	case 2:
		// 第二个返回值必须实现 error 接口
		if !rt.Out(1).Implements(errorType) {
			return nil, errInvalidFactory
		}
	default:
		return nil, errInvalidFactory
	}
	concreteType := rt.Out(0)
	// 检查是否循环引用
	for i := 0; i < rt.NumIn(); i++ {
		if rt.In(i) == concreteType { // 循环依赖
			return nil, errCircularReference
		}
	}
	b := &binding{
		name:    name,
		typ:     concreteType,
		factory: rv,
	}
	if len(shared) > 0 {
		b.shared = shared[0]
	}
	return b, nil
}

func (b *binding) make(c *Container) (reflect.Value, error) {
	if values, exists := c.instances[b.typ]; exists {
		v, ok := values[b.name]
		if ok {
			return v, nil
		}
	}
	val, err := c.invoke(b.factory.Type(), b.factory)
	if err != nil {
		return reflect.Value{}, err
	}
	rv := val[0]
	if len(val) == 2 {
		err = val[1].Interface().(error)
		if err != nil {
			return reflect.Value{}, err
		}
	}
	if b.shared && rv.IsValid() {
		c.setInstance(b.name, b.typ, rv)
	}
	return rv, nil
}
