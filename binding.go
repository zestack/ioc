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
	typ     reflect.Type
	factory reflect.Value
	shared  bool
}

func newBinding(factory any, shared ...bool) (*binding, error) {
	factoryReflector := reflect.ValueOf(factory)
	factoryType := factoryReflector.Type()
	if factoryType.Kind() != reflect.Func {
		return nil, errNotFactory
	}
	switch returnCount := factoryType.NumOut(); returnCount {
	case 1:
		// 只有一个返回值
	case 2:
		// 第二个返回值必须实现 error 接口
		if !factoryType.Out(1).Implements(errorType) {
			return nil, errInvalidFactory
		}
	default:
		return nil, errInvalidFactory
	}
	concreteType := factoryType.Out(0)
	// 检查是否循环引用
	for i := 0; i < factoryType.NumIn(); i++ {
		if factoryType.In(i) == concreteType { // 循环依赖
			return nil, errCircularReference
		}
	}
	b := &binding{
		typ:     concreteType,
		factory: factoryReflector,
	}
	if len(shared) > 0 {
		b.shared = shared[0]
	}
	return b, nil
}

func (b *binding) make(c *Container) (reflect.Value, error) {
	if v, exists := c.instances[b.typ]; exists {
		return v, nil
	}
	val, err := c.invoke(b.typ, b.factory)
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
		c.setInstance(b.typ, rv)
	}
	return rv, nil
}
