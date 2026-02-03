package main

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/cosmorse/protoc-gen-ego/testdata"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
)

func TestRun(t *testing.T) {
	os.Args = []string{"protoc-gen-test"}
	camelcaseEnumConstants = true
	file, err := os.Open("testdata/message.desc")
	if err != nil {
		t.Error(err)
		return
	}
	defer file.Close()

	opts := &protogen.Options{}
	err = run(file, opts)
	if err != nil {
		t.Error(err)
	}
}

func TestParseGoSource(t *testing.T) {
	in, err := os.Open("testdata/message.pb.go")
	if err != nil {
		t.Error(err)
		return
	}
	defer in.Close()

	src, err := io.ReadAll(in)
	if err != nil {
		t.Error(err)
		return
	}

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "", src, parser.ParseComments)
	if err != nil {
		t.Error(err)
		return
	}

	var objName string
	ast.Inspect(file, func(node ast.Node) bool {
		switch obj := node.(type) {
		case *ast.TypeSpec:
			if obj.Name != nil && obj.Name.Name == "Message" {
				objName = obj.Name.Name
			}
		case *ast.StructType:
			for _, field := range obj.Fields.List {

				t.Log(objName)
				if field.Tag != nil && len(field.Names) == 1 && field.Names[0].String() == "Id" {
					field.Tag.Value = strings.Replace(field.Tag.Value, "id,omitempty", "_id", 1)
					field.Names[0].Name = "TEST_ID"
				}
			}
		}

		return true
	})

	err = format.Node(os.Stdout, fileSet, file)
	if err != nil {
		t.Error(err)
		return
	}

}

func TestGeneration(t *testing.T) {
	msg := &testdata.Message{
		Id:    1,
		Quote: []byte("123"),
		Role:  testdata.GroupRoleCreator,
	}

	msg.Pet = &testdata.Message_Cat{
		Cat: &testdata.Cat{Name: "cat"},
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		t.Error(err)
		return
	}

	var msg2 testdata.Message
	err = proto.Unmarshal(data, &msg2)
	if err != nil {
		t.Error(err)
	}

	t.Log(&msg2)
}
