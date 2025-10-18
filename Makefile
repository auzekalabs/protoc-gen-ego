SOURCES=$(shell find . -name "*.go")
TEST_PROTO_FILES=$(shell find testdata -name "*.proto")

protoc-gen-ego: $(SOURCES)
	go build -ldflags '-s -w' -o $@ .


.PHONY: install
install: $(SOURCES)
	go install -ldflags '-s -w -X main.version=$(VERSION) -X main.rc=$(RC)'

.PHONY: desc
desc: ./testdata/message.proto
	protoc --proto_path=./testdata --plugin=protoc-gen-ego=./gen_desc --ego_out=paths=source_relative,enum=camelcase:./testdata message.proto

.PHONY: test
test: protoc-gen-ego $(TEST_PROTO_FILES)
	protoc --proto_path=. --plugin=protoc-gen-ego=./protoc-gen-ego --ego_out=paths=source_relative,enum=camelcase:. $(TEST_PROTO_FILES)

.PHONY: clean
clean:
	@rm -f protoc-gen-ego