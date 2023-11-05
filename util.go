package ioc

import (
	"reflect"
	"strings"
)

// InterfaceOf dereferences a pointer to an Interface type.
// It panics if a value is not a pointer to an interface.
func InterfaceOf(value any) reflect.Type {
	t := reflect.TypeOf(value)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Interface {
		panic("the value is not a pointer to an interface. (*MyInterface)(nil)")
	}
	return t
}

func parseTag(field reflect.StructField) (name string, omitempty, inject bool) {
	if name, inject = field.Tag.Lookup(tagName); inject {
		segments := strings.Split(name, ",")
		for i := 0; i < len(segments); i++ {
			if i == 0 {
				name = segments[0]
			} else if segments[i] == "omitempty" {
				omitempty = true
				break
			}
		}
	}
	return
}
