# go-c2go

Generate Go-callable Plan 9 assembly from small nolibc C functions on `amd64` and `arm64`.

## Usage

```go
package sample

//go:generate go run github.com/frankli0324/go-c2go/cmd/c2go -c sample.c
```

```bash
go generate ./...
go test ./...
```

`-c` is explicit and repeatable. No automatic `*.c` scan.

Generated from `sample.c`:

- `sample_c2go.go`
- `sample_c2go_${GOARCH}.s`

## Commands

```bash
go run github.com/frankli0324/go-c2go/cmd/c2go -c sample.c
go run ./cmd/c2go -src sample.c -pkg sample -go sample.go
go run ./cmd/asm2go -src sample.s -syntax att
go run ./cmd/asm2go -src sample.s -syntax intel
```

Useful flags:

- `-arch`: defaults to `GOARCH`; supports `amd64` and `arm64`.
- `-syntax`: `auto`, `att`, `intel`, `plan9`; `att/intel` are amd64-only, `arm64` uses `auto`, `plan9` is not implemented.
- `-cc`: C compiler, defaults to `clang`.
- `-cflag`: extra compiler flag, repeatable.
- `-o`: optional asm output path; default is `<src>_<GOARCH>.s`.

## C API Shape

Supported scalar mappings are selected from the target C ABI:

```text
char/signed char                  -> int8
unsigned char                     -> uint8
short/signed short                -> target-width signed integer
unsigned short                    -> target-width unsigned integer
int/signed int                    -> target-width signed integer
unsigned int                      -> target-width unsigned integer
long                              -> target-width signed integer
unsigned long                     -> target-width unsigned integer
long long                         -> int64
unsigned long long                -> uint64
size_t                            -> uint
void*/const void*                 -> unsafe.Pointer
void                              -> no return / no params
```

`[]byte` input:

```c
int first(const unsigned char *buf, size_t buf_len);
```

becomes:

```go
func First(buf []byte) int32
```

`const char *` and `const unsigned char *` returns are rejected.

## Constraints

- C only. No C++, templates, namespaces, overloads, or name mangling.
- Nolibc: default flags include `-ffreestanding -fno-builtin -fno-stack-protector`.
- Do not call libc symbols like `malloc`, `memcpy`, or `printf`.
- No structs, floating point, or general pointers.
- Max integer-register C ABI arguments: six on `amd64`, eight on `arm64`; `[]byte` counts as two.
- Unsupported asm lines become `// UNSUPPORTED: ...`.
