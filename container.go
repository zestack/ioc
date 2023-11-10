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

// Container 服务容器
// TODO(hupeh): 保证并发安全
type Container struct {
	parent    *Container
	factories map[reflect.Type]map[string]*binding
	instances map[reflect.Type]map[string]reflect.Value
}

// New 新建一个服务容器
func New() *Container {
	return &Container{}
}

// Fork 派生出一个子容器，该子容器能够通过父子关系远程
// 使用父容器里面的服务，因此同时可以安全的设置与父容器
// 一致的服务而不影响父容器。
func (c *Container) Fork() *Container {
	return &Container{parent: c}
}

// Bind 绑定一个“具体实现”（实例或原语值），需要注意的是，由于内部
// 是根据类型与“具体实现”直接建立映射关系的，因此同一种类型最多只会
// 有一个具体实现。
func (c *Container) Bind(value any) {
	c.NamedBind("", value)
}

// NamedBind 具名绑定一个“具体实现”（实例或原语值），由于这个“具体实现”拥有了
// 名称，所以不会覆盖掉通过 Bind 方法绑定的“具体实现”，这也能够解决同一种类型
// 在不同的场景和用途下可以指定不同的“具体实现”，因此我们的结构体可以通过指定 `ioc`
// 这个 tag 实现依赖注入时选择我们绑定的“具体实现”。
func (c *Container) NamedBind(name string, value any) {
	rt := reflect.TypeOf(value)
	rv := reflect.ValueOf(value)
	c.setInstance(name, rt, rv)
}

// 提示：不能通过第三个参数来推导出第二个参数！！！
func (c *Container) setInstance(name string, rt reflect.Type, rv reflect.Value) {
	if c.instances == nil {
		c.instances = make(map[reflect.Type]map[string]reflect.Value)
	}
	if _, ok := c.instances[rt]; !ok {
		c.instances[rt] = make(map[string]reflect.Value)
	}
	c.instances[rt][name] = rv
}

// Factory 绑定一个工厂函数，工厂函数必须返回一个“具体实现”，同时还可以返回一个错误对象
// 表示构建失败，该方法的实现方式与 Bind 方法类似，同一种类型最多也只会有一个工厂函数。
func (c *Container) Factory(factory any, shared ...bool) error {
	return c.NamedFactory("", factory, shared...)
}

// NamedFactory 具名绑定工厂函数，该方法的实现方式与 NamedBind 方法类型。
func (c *Container) NamedFactory(name string, factory any, shared ...bool) error {
	b, err := newBinding(name, factory, shared...)
	if err != nil {
		return err
	}
	if c.factories == nil {
		c.factories = make(map[reflect.Type]map[string]*binding)
	}
	if _, ok := c.factories[b.typ]; !ok {
		c.factories[b.typ] = make(map[string]*binding)
	}
	c.factories[b.typ][name] = b
	return nil
}

// Get 获取指定类型的“具体实现”值，获取步骤如下：
// * 1、使用事先通过 Bind 方法绑定了值；
// * 2、执行 Factory 方法绑定的工厂函数；
// * 3、若无法通过上述途径获取，且类型是结构体或结构体指针时，尝试构建一个实例。
func (c *Container) Get(t reflect.Type) (reflect.Value, error) {
	return c.get("", t)
}

// NamedGet 具名方式获取指定类型的“具体实现”值，该方法与 Get 类似。
func (c *Container) NamedGet(name string, t reflect.Type) (reflect.Value, error) {
	return c.get(name, t)
}

func (c *Container) get(name string, t reflect.Type) (reflect.Value, error) {
	if t == nil {
		return reflect.Value{}, ErrValueNotFound
	}
	// 获取通过 Bind 或 NamedBind 绑定的值
	values, instanced := c.instances[t]
	if instanced {
		value, exists := values[name]
		if exists && value.IsValid() {
			return value, nil
		}
	}
	// 通过执行 Factory 或 NamedFactory 绑定的工厂函数获取值
	bindings, bound := c.factories[t]
	if bound {
		bind, exists := bindings[name]
		if exists {
			val, err := bind.make(c)
			if err != nil {
				// TODO(hupeh): 更加友好的错误信息
				return reflect.Value{}, err
			}
			if val.IsValid() {
				return val, nil
			}
		}
	}

	// 使用同名但不同类型里面可以被转换或被实现的
	for rt, values := range c.instances {
		if rt != t && (t.Kind() == reflect.Interface && rt.Implements(t) || rt.AssignableTo(t)) {
			val, ok := values[name]
			if ok {
				return val, nil
			}
		}
	}
	// 查看注的册工厂函数，看它们的具体实现是否可以被转换或被实现的
	for rt, bindings := range c.factories {
		if t != rt {
			switch {
			case t.Kind() == reflect.Interface && rt.Implements(t):
			case rt.AssignableTo(t):
				bind, ok := bindings[""]
				if !ok {
					continue
				}
				val, err := bind.make(c)
				if err != nil {
					continue
				}
				if val.IsValid() {
					return val, nil
				}
			}
		}
	}

	if c.parent != nil {
		return c.NamedGet(name, t)
	}

	rt := t
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}

	// 如果给的是结构体，则直接构建
	if t.Kind() == reflect.Struct {
		rv := reflect.New(t)
		err := c.resolve(&rv)
		if err != nil {
			return reflect.Value{}, err
		}
		// TODO(hupeh): 对于结构体指针怎么处理？
		return rv, nil
	}

	return reflect.Value{}, ErrValueNotFound
}

// Resolve 依赖注入，在结构体中，可以通过指定一个名为 ioc 的 tag 表明
// 使用的指定的名称的“具体实现”来完成注入。
func (c *Container) Resolve(i any) error {
	v := reflect.ValueOf(i)
	return c.resolve(&v)
}

func (c *Container) resolve(rv *reflect.Value) error {
	v := *rv
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
	rv = &v
	return nil
}

// Invoke 执行指定的函数，使用服务容器完成参数注入。
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

// NewContext 返回一个被注入的服务容器的上下
func (c *Container) NewContext(parentCtx ...context.Context) context.Context {
	for _, ctx := range parentCtx {
		if ctx != nil {
			return context.WithValue(ctx, contextKey, c)
		}
	}
	return context.WithValue(context.Background(), contextKey, c)
}
