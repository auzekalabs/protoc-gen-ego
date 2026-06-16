# protoc-gen-ego

The protoc-gen-ego project is another protoc plugin to generate Go code for both proto2 and proto3 versions of the protocol buffer language. 

The protoc-gen-ego works just like official plugin, and provides some new features to auto generate message field tag and customized field name from comments, and CamelCase formatted enum constants name 

For more information about the usage of this plugin, see: https://protobuf.dev/reference/go/go-generated.

## Features
- auto generate message field tags from comments
- customize message field name
- generate CamelCase formatted enum constants name

## Limitations
- only works for message field declaration

## Installation
```bash
go install github.com/auzekalabs/proto-gen-ego
```

## Usage
### General Command Line Arguments
```bash
protoc --plugin=protoc-gen-ego --ego_out=. --ego_opt=paths=source_relative xxx/xxx.proto
```

### Generate CamelCase Enum Constants
- protobuf source code
```protobuf
enum Role {
  UNSPECIFIC      = 0;
  CREATOR         = 1;
  OWNER           = 2;
  FINANCE_MANAGER = 3;
}
```
- generate option
```bash
protoc --plugin=protoc-gen-ego --ego_out=enum=camelcase:. --ego_opt=paths=source_relative xxx/xxx.proto
```

- generated go source code
```go
type Role int32

const (
	RoleUnspecific     Role = 0
	RoleCreator        Role = 1
	RoleOwner          Role = 2
	RoleFinanceManager Role = 3
)
```

### Customize Enum Name
- protobuf source code
using `@go.enum=strip` to strip the enum type name
```protobuf
// @go.enum=strip
enum OS {
    HARMONY = 0;
    // @go.name=IOS
    IOS     = 1;
    ANDROID = 2;
}
```

- generated go source code
```go
type OS int32
const (
    Harmony OS = 0
    IOS     OS = 1
    Android OS = 2
)
```

### Generate Field Tags
    
- protobuf source code
```protobuf
message foo {
    // @gorm.tag=column:id;autoIncrement
    // @json.tag=ID
    int64 id = 1;
}
```
- generated go source code
```go
type Foo struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields

    Id int64 `protobuf:"varint,1,opt,name=id,proto3" json:"ID,omitempty" gorm:"column:id;autoIncrement"`
}
```

### Customize Field Name
- protobuf source code
```protobuf
message foo {
    // @go.name=ID
    int64 id = 1;
}
```
- generated go source code
```go
type Foo struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields

    ID int64 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
}
```
