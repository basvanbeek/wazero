# wazero: the zero dependency WebAssembly runtime for Go developers

[![WebAssembly Core Specification Test](https://github.com/tetratelabs/wazero/actions/workflows/spectest.yaml/badge.svg)](https://github.com/tetratelabs/wazero/actions/workflows/spectest.yaml) [![Go Reference](https://pkg.go.dev/badge/github.com/tetratelabs/wazero.svg)](https://pkg.go.dev/github.com/tetratelabs/wazero) [![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

WebAssembly is a way to safely run code compiled in other languages. Runtimes
execute WebAssembly Modules (Wasm), which are most often binaries with a `.wasm`
extension.

wazero is a WebAssembly Core Specification [1.0][1] and [2.0][2] compliant
runtime written in Go. It has *zero dependencies*, and doesn't rely on CGO.
This means you can run applications in other languages and still keep cross
compilation.

Import wazero and extend your Go application with code written in any language!

## Example

The best way to learn wazero is by trying one of our [examples](examples).

For the impatient, here's a peek of a general flow with wazero:

First, you need to compile your code into the WebAssembly Binary Format (Wasm).

Here's source in [TinyGo](https://wazero.io/languages/tinygo), which exports an
"add" function:
```go
package main

//export add
func add(x, y uint32) uint32 {
	return x + y
}
```

Here's the minimal command to build a `%.wasm` binary.
```bash
tinygo build -o add.wasm -target=wasi add.go
```

Finally, you can run that inside your Go application.
```go
func main() {
	// Choose the context to use for function calls.
	ctx := context.Background()

	// Read a WebAssembly binary containing an exported "add" function.
	wasm, err := os.ReadFile("./path/to/add.wasm")
	if err != nil {
		log.Panicln(err)
	}

	// Create a new WebAssembly Runtime.
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		// WebAssembly 2.0 allows use of any version of TinyGo, including 0.24+.
		WithWasmCore2())
	defer r.Close(ctx) // This closes everything this Runtime created.

	// Instantiate WASI, which implements system I/O such as console output.
	if _, err = wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		log.Panicln(err)
	}

	// Instantiate the module and return its exported functions
	module, err := r.InstantiateModuleFromBinary(ctx, wasm)
	if err != nil {
		log.Panicln(err)
	}

	// Discover 1+2=3
	fmt.Println(module.ExportedFunction("add").Call(ctx, 1, 2))
}
```

Notes:

* The embedding application is often called the "host" in WebAssembly.
* The Wasm binary is often called the "guest" in WebAssembly. Sometimes they
  need [imports][imports] to implement features such as console output.
* Many languages compile to (target) Wasm including AssemblyScript, C, C++,
  Rust, TinyGo and Zig!

## Deeper dive

The former example is a pure function. While a good start, you probably are
wondering how to do something more realistic, like read a file. WebAssembly
Modules (Wasm) are sandboxed similar to containers. They can't read anything
on your machine unless you explicitly allow it.

The WebAssembly Core Specification is a standard, governed by W3C process, but
it has no scope to specify how system resources like files are accessed.
Instead, WebAssembly defines "host functions" and the signatures they can use.
In wazero, "host functions" are written in Go, and let you do anything
including access files. The main constraint is that WebAssembly only allows
numeric types. wazero includes [imports][imports] for common languages and
compiler toolchains.

For example, you can grant WebAssembly code access to your console by exporting
a function written in Go. The below function can be imported into standard
WebAssembly as the module "env" and the function name "log_i32".
```go
_, err := r.NewModuleBuilder("env").
	ExportFunction("log_i32", func(v uint32) {
		fmt.Println("log_i32 >>", v)
	}).
	Instantiate(ctx, r)
if err != nil {
	log.Panicln(err)
}
```

The WebAssembly community has [subgroups][4] which maintain work that may not
result in a Web Standard. One such group is the WebAssembly System Interface
([WASI][5]), which defines functions similar to Go's [x/sys/unix][6].

The [wasi_snapshot_preview1][13] tag of WASI is widely implemented, so wazero
bundles an implementation. That way, you don't have to write these functions.

For example, here's how you can allow WebAssembly modules to read
"/work/home/a.txt" as "/a.txt" or "./a.txt" as well the system clock:
```go
_, err := wasi_snapshot_preview1.Instantiate(ctx, r)
if err != nil {
	log.Panicln(err)
}

config := wazero.NewModuleConfig().
	WithFS(os.DirFS("/work/home")). // instead of no file system
	WithSysWalltime().WithSysNanotime() // instead of fake time

module, err := r.InstantiateModule(ctx, compiled, config)
...
```

While we hope this deeper dive was useful, we also provide [examples](examples)
to elaborate each point. Please try these before raising usage questions as
they may answer them for you!

## Runtime

There are two runtime configurations supported in wazero: _Compiler_ is default:

By default, ex `wazero.NewRuntime(ctx)`, the Compiler is used if supported. You
can also force the interpreter like so:
```go
r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
```

### Interpreter
Interpreter is a naive interpreter-based implementation of Wasm virtual
machine. Its implementation doesn't have any platform (GOARCH, GOOS) specific
code, therefore _interpreter_ can be used for any compilation target available
for Go (such as `riscv64`).

### Compiler
Compiler compiles WebAssembly modules into machine code ahead of time (AOT),
during `Runtime.CompileModule`. This means your WebAssembly functions execute
natively at runtime. Compiler is faster than Interpreter, often by order of
magnitude (10x) or more. This is done without host-specific dependencies.

If interested, check out the [RATIONALE.md][8] and help us optimize further!

### Conformance

Both runtimes pass WebAssembly Core [1.0][7] and [2.0][14] specification tests
on supported platforms:

| Runtime | Usage| amd64 | arm64 | others |
|:---:|:---:|:---:|:---:|:---:|
| Interpreter|`wazero.NewRuntimeConfigInterpreter()`|✅ |✅|✅|
| Compiler |`wazero.NewRuntimeConfigCompiler()`|✅|✅ |❌|

## Support Policy

The below support policy focuses on compatability concerns of those embedding
wazero into their Go applications.

### wazero

wazero is an early project, so APIs are subject to change until version 1.0.

Wazero 1.0 will have a floor Go version of 1.19. The first beta will be the
end of August 2022, and the final release will be after Go 1.20 is released,
in February 2023.

Meanwhile, please practice the current APIs to ensure they work for you!

### Go

wazero has no dependencies except Go, so the only source of conflict in your
project's use of wazero is the Go version.

To simplify our support policy, we adopt Go's [Release Policy][10] (two versions).

This means wazero will remain compilable and tested on the version prior to the
latest release of Go.

For example, once Go 1.29 is released, wazero may use a Go 1.28 feature.

### Platform

wazero has two runtime modes: Interpreter and Compiler. The only supported operating
systems are ones we test, but that doesn't necessarily mean other operating
system versions won't work.

We currently test Linux (Ubuntu and scratch), MacOS and Windows as packaged by
[GitHub Actions][11], as well FreeBSD via Vagrant/VirtualBox.

* Interpreter
  * Linux is tested on amd64 (native) as well arm64 and riscv64 via emulation.
  * FreeBSD, MacOS and Windows are only tested on amd64.
* Compiler
  * Linux is tested on amd64 (native) as well arm64 via emulation.
  * FreeBSD, MacOS and Windows are only tested on amd64.

wazero has no dependencies and doesn't require CGO. This means it can also be
embedded in an application that doesn't use an operating system. This is a main
differentiator between wazero and alternatives.

We verify zero dependencies by running tests in Docker's [scratch image][12].
This approach ensures compatibility with any parent image.

-----
wazero is a registered trademark of Tetrate.io, Inc. in the United States and/or other countries

[1]: https://www.w3.org/TR/2019/REC-wasm-core-1-20191205/
[2]: https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/
[4]: https://github.com/WebAssembly/meetings/blob/main/process/subgroups.md
[5]: https://github.com/WebAssembly/WASI
[6]: https://pkg.go.dev/golang.org/x/sys/unix
[7]: https://github.com/WebAssembly/spec/tree/wg-1.0/test/core
[8]: internal/engine/compiler/RATIONALE.md
[9]: https://github.com/tetratelabs/wazero/issues/506
[10]: https://go.dev/doc/devel/release
[11]: https://github.com/actions/virtual-environments
[12]: https://docs.docker.com/develop/develop-images/baseimages/#create-a-simple-parent-image-using-scratch
[13]: https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md
[14]: https://github.com/WebAssembly/spec/tree/d39195773112a22b245ffbe864bab6d1182ccb06/test/core
