package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"

	"github.com/samber/lo"
	"github.com/stoewer/go-strcase"
	gengo "google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

var camelcaseEnumConstants = false

// run executes a function as a protoc plugin.
//
// It reads a [pluginpb.CodeGeneratorRequest] message from [os.Stdin], invokes the plugin
// function, and writes a [pluginpb.CodeGeneratorResponse] message to [os.Stdout].
//
// If a failure occurs while reading or writing, Run prints an error to
// [os.Stderr] and calls [os.Exit](1).
func run(reader io.Reader, opts *protogen.Options) error {
	if len(os.Args) > 1 {
		return fmt.Errorf("unknown argument %q (this program should be run by protoc, not directly)", os.Args[1])
	}

	in, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	req := &pluginpb.CodeGeneratorRequest{}
	if err = proto.Unmarshal(in, req); err != nil {
		return err
	}

	if param := req.GetParameter(); strings.Contains(param, "enum=camelcase") {
		camelcaseEnumConstants = true
	}

	gen, err := opts.New(req)
	if err != nil {
		return err
	}

	descs, err := refactorProtoSources(gen.Files)
	if err != nil {
		return err
	}

	gen.SupportedFeatures = gengo.SupportedFeatures
	gen.SupportedEditionsMinimum = gengo.SupportedEditionsMinimum
	gen.SupportedEditionsMaximum = gengo.SupportedEditionsMaximum

	for _, f := range gen.Files {
		if f.Generate {
			gengo.GenerateFile(gen, f)
		}
	}

	resp := gen.Response()

	if err = regenerateGoSources(descs, resp.File); err != nil {
		return err
	}

	out, err := proto.Marshal(resp)
	if err != nil {
		return err
	}

	if _, err = os.Stdout.Write(out); err != nil {
		return err
	}

	return nil
}

func refactorProtoSources(files []*protogen.File) (descs []*FileDescriptor, err error) {
	descs = make([]*FileDescriptor, 0, len(files))
	for _, file := range files {
		if !file.Generate || file.Desc == nil {
			continue
		}

		if err = refactorEnumConstants(file.Enums); err != nil {
			return nil, err
		}

		desc := &FileDescriptor{
			ProtoPath: file.Desc.Path(),
			GoPath:    strings.Replace(file.Desc.Path(), ".proto", ".pb.go", 1),
			Models:    make(map[string]*Model),
		}

		if err = desc.parse(file); err != nil {
			return
		}

		if len(desc.Models) > 0 {
			descs = append(descs, desc)
		}
	}

	return
}

const (
	stripNone = iota
	stripNested
	stripAll
)

func refactorEnumConstants(enums []*protogen.Enum, prefixes ...string) error {
	if !camelcaseEnumConstants {
		return nil
	}

	var (
		prefix   string
		enumType string
	)

	if len(prefixes) > 0 {
		prefix = prefixes[0]
	}

	for _, enum := range enums {
		enumType = strings.TrimPrefix(enum.GoIdent.GoName, prefix)
		strip := renameEnumType(enum)
		for _, value := range enum.Values {
			renameEnumValue(value, prefix, enumType, strip)
		}
	}

	return nil
}

func renameEnumType(enum *protogen.Enum) (strip int) {
	const (
		stripComment  = "@go.enum=strip"
		goNameComment = "@go.name="
	)

	var name string

	leading := enum.Comments.Leading.String()
	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(leading))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if index := strings.Index(line, stripComment); index >= 0 {
			line = line[index+len(stripComment):]
			if strings.HasPrefix(line, "=nested") {
				strip = stripNested
			} else {
				strip = stripAll
			}

			continue
		}

		if index := strings.Index(line, goNameComment); index >= 0 {
			name = strings.TrimSpace(line[index+len(goNameComment):])
			continue
		}

		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	enum.Comments.Leading = protogen.Comments(buf.String())

	trailing := enum.Comments.Trailing.String()
	if index := strings.Index(trailing, stripComment); index >= 0 {
		trailing = trailing[index+len(stripComment):]
		if strings.HasPrefix(trailing, "=nested") {
			strip = stripNested
		} else {
			strip = stripAll
		}
		enum.Comments.Trailing = ""
	} else if index := strings.Index(trailing, goNameComment); index >= 0 {
		enum.Comments.Trailing = ""
		name = strings.TrimSpace(trailing[index+len(goNameComment):])
	}

	if name != "" {
		enum.GoIdent.GoName = name
	} else {
		enum.GoIdent.GoName = strcase.UpperCamelCase(enum.GoIdent.GoName)
	}

	return
}

func renameEnumValue(value *protogen.EnumValue, prefix, enum string, strip int) {
	var (
		name string
		buf  bytes.Buffer
	)

	const goNameComment = "@go.name="

	scanner := bufio.NewScanner(strings.NewReader(value.Comments.Leading.String()))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if index := strings.Index(line, goNameComment); index >= 0 {
			name = strings.TrimSpace(line[index+len(goNameComment):])
			continue
		}

		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	value.Comments.Leading = protogen.Comments(buf.String())
	if name == "" {
		trailing := value.Comments.Trailing.String()
		if index := strings.Index(trailing, goNameComment); index >= 0 {
			name = strings.TrimSpace(trailing[index+len(goNameComment):])
			value.Comments.Trailing = ""
		}
	}

	switch strip {
	case stripAll:
		if name != "" {
			value.GoIdent.GoName = name
			return
		}

		if prefix != "" {
			value.GoIdent.GoName = strings.TrimPrefix(value.GoIdent.GoName, prefix+"_")
		} else {
			value.GoIdent.GoName = strings.TrimPrefix(value.GoIdent.GoName, enum+"_")

		}
	case stripNested:
		if name != "" {
			value.GoIdent.GoName = strcase.UpperCamelCase(enum) + name
			return
		} else {
			value.GoIdent.GoName = strings.TrimPrefix(value.GoIdent.GoName, prefix+"_")
			value.GoIdent.GoName = enum + "_" + value.GoIdent.GoName
		}
	default:
		if name != "" {
			if prefix != "" {
				value.GoIdent.GoName = strcase.UpperCamelCase(prefix) + name
			} else {
				value.GoIdent.GoName = strcase.UpperCamelCase(enum) + name
			}

			return
		}
	}

	value.GoIdent.GoName = strcase.UpperCamelCase(value.GoIdent.GoName)
}

func regenerateGoSources(descs []*FileDescriptor, sources []*pluginpb.CodeGeneratorResponse_File) (err error) {
	if len(descs) == 0 || len(sources) == 0 {
		return
	}

	for _, source := range sources {
		desc, ok := lo.Find(descs, func(desc *FileDescriptor) bool {
			return desc.GoPath == *source.Name
		})

		if !ok || desc == nil {
			continue
		}

		var newSource string
		if newSource, err = generate(*source.Content, desc); err != nil {
			return
		}

		if len(newSource) > 0 {
			source.Content = &newSource
		}
	}

	return
}

func generate(source string, desc *FileDescriptor) (string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "", source, parser.ParseComments)
	if err != nil {
		return "", err
	}

	ast.Inspect(file, func(node ast.Node) bool {
		switch spec := node.(type) {
		case *ast.TypeSpec:
			if spec.Name == nil {
				break
			}

			model, ok := desc.Models[spec.Name.Name]
			if !ok || model == nil {
				break
			}

			if stype, ok := spec.Type.(*ast.StructType); ok && stype != nil {
				for _, field := range stype.Fields.List {
					if len(field.Names) != 1 || field.Tag == nil {
						continue
					}

					fdesc, ok := model.Fields[field.Names[0].String()]
					if !ok || fdesc == nil || len(fdesc.Tags) == 0 {
						continue
					}

					var value bytes.Buffer
					value.WriteString(field.Tag.Value[:len(field.Tag.Value)-1])

					for _, tag := range fdesc.Tags {
						switch strings.ToLower(tag.Kind) {
						case "json":
							if !strings.Contains(tag.Value, "omitempty") {
								tag.Value += ",omitempty"
							}

							newTag := "json:\"" + tag.Value + "\""
							original := value.String()
							if begin := strings.Index(original, "json:\""); begin >= 0 {
								if end := strings.Index(original[begin+7:], "\""); end > 0 {
									original = strings.Replace(original, original[begin:begin+7+end+1], newTag, 1)
									value.Reset()
									value.WriteString(original)
								}
							} else {
								value.WriteByte(' ')
								value.WriteString(newTag)
							}
						default:
							value.WriteByte(' ')
							value.WriteString(tag.Kind)
							value.WriteString(":\"")
							value.WriteString(tag.Value)
							value.WriteString("\"")
						}
					}

					value.WriteByte('`')
					field.Tag.Value = value.String()
				}
			}
		}
		return true
	})

	buf := bytes.NewBuffer(make([]byte, 0, len(source)*2))

	if err = format.Node(buf, fileSet, file); err != nil {
		return "", err
	}

	return buf.String(), err
}
