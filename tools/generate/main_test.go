package main

import (
	"reflect"
	"testing"

	"github.com/zeromicro/go-zero/tools/goctl/api/spec"
)

func TestSelectTypesKeepsOnlyAllowedRouteTypeClosure(t *testing.T) {
	types := []spec.Type{
		spec.DefineStruct{RawName: "Request", Members: []spec.Member{{Name: "Payload", Type: spec.DefineStruct{RawName: "Payload"}}}},
		spec.DefineStruct{RawName: "Payload", Members: []spec.Member{{Name: "Items", Type: spec.ArrayType{RawName: "[]Item", Value: spec.DefineStruct{RawName: "Item"}}}}},
		spec.DefineStruct{RawName: "Response", Members: []spec.Member{{Name: "Item", Type: spec.DefineStruct{RawName: "Item"}}}},
		spec.DefineStruct{RawName: "Item"},
		spec.DefineStruct{RawName: "AdminOnly"},
	}

	selected, err := selectTypes(types, []routeDefinition{{
		RequestType:  spec.DefineStruct{RawName: "Request"},
		ResponseType: spec.DefineStruct{RawName: "Response"},
	}})
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, len(selected))
	for index, typ := range selected {
		names[index] = typ.Name()
	}
	if want := []string{"Request", "Payload", "Response", "Item"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("selected types = %v, want %v", names, want)
	}
}
