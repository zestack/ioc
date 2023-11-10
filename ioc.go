package ioc

import (
	"context"
	"errors"
	"reflect"
)

var global = New()

// Fork 分支
func Fork() *Container {
	return global.Fork()
}

// Bind 绑定值到容器，有效类型：
//
// - 接口的具体实现值
// - 结构体的实例
// - 类型的值（尽量不要使用原始类型，而应该使用元素类型的变体）
func Bind(instance any) {
	global.Bind(instance)
}

// NamedBind 绑定具名值到容器
func NamedBind(name string, instance any) {
	global.NamedBind(name, instance)
}

// Factory 绑定工厂函数
func Factory(factory any, shared ...bool) error {
	return global.Factory(factory, shared...)
}

func MustFactory(factory any, shared ...bool) {
	err := Factory(factory, shared...)
	if err != nil {
		panic(err)
	}
}

// NamedFactory 绑定具名工厂函数
func NamedFactory(name string, factory any, shared ...bool) error {
	return global.NamedFactory(name, factory, shared...)
}

func MustNamedFactory(name string, factory any, shared ...bool) {
	err := NamedFactory(name, factory, shared...)
	if err != nil {
		panic(err)
	}
}

// Resolve 完成的注入
func Resolve(i any) error {
	return global.Resolve(i)
}

// Get 获取指定类型的值，泛型 T 只能是结构体
//
// 如果需要获取一个接口的实例，我们可以使用 Instance 函数
// 返回一个 Container 实例，然后通过改容器来获取接口的具体实现。
func Get[T any](ctx context.Context) (*T, error) {
	return NamedGet[T](ctx, "")
}

// MustGet 获取指定类型的值，泛型 T 只能是结构体。
//
// 如果需要获取一个接口的实例，我们可以使用 Instance 函数
// 返回一个 Container 实例，然后通过改容器来获取接口的具体实现。
func MustGet[T any](ctx context.Context) *T {
	return MustNamedGet[T](ctx, "")
}

// NamedGet 通过注入的名称获取指定类型的值
func NamedGet[T any](ctx context.Context, name string) (*T, error) {
	var abstract T
	t := reflect.TypeOf(&abstract)
	if ctx != nil {
		if ci, ok := ctx.Value(contextKey).(*Container); ok {
			val, err := ci.NamedGet(name, t)
			if err != nil {
				if !errors.Is(err, ErrValueNotFound) {
					return nil, err
				}
			} else if val.IsValid() {
				return val.Interface().(*T), nil
			}
		}
	}
	val, err := global.NamedGet(name, t)
	if err != nil {
		return nil, err
	}
	if !val.IsValid() {
		return nil, ErrValueNotFound
	}
	return val.Interface().(*T), nil
}

func MustNamedGet[T any](ctx context.Context, name string) *T {
	v, err := NamedGet[T](ctx, name)
	if err != nil {
		panic(err)
	}
	return v
}

// Invoke 执行函数
func Invoke(f any) ([]reflect.Value, error) {
	return global.Invoke(f)
}

func NewContext(parentCtx ...context.Context) context.Context {
	return global.NewContext(parentCtx...)
}

func Instance(ctx context.Context) *Container {
	if ctx == nil {
		return global
	}
	if ci, ok := ctx.Value(contextKey).(*Container); ok {
		return ci
	}
	return global
}
