# go-rime

go-rime provides Go bindings to the [librime](https://github.com/rime/librime) input method engine. It loads a native shared library at runtime through platform loaders and calls the public C API through [purego](https://github.com/ebitengine/purego). Building a Go application requires Go 1.25 or newer and supports `CGO_ENABLED=0`.

## Installation

The target machine needs librime for its OS and CPU architecture, any native dependencies of that build, and Rime data files for the schemas the application uses.

| Platform | Go targets | Native library and data |
| --- | --- | --- |
| Windows | `amd64`, `arm64` | Use `rime.dll` from a matching [librime release or build](https://github.com/rime/librime/blob/master/README-windows.md). Keep its dependent DLLs available to the loader. Supply schema and dictionary files separately. |
| Linux | `amd64`, `arm64` | Install your distribution's shared librime package and Rime data. On Debian and Ubuntu, `apt install librime-dev rime-data-luna-pinyin` supplies the library, headers and an example schema. The usual shared-data directory is `/usr/share/rime-data`. |
| macOS | `amd64`, `arm64` | `brew install librime` supplies the [native library](https://formulae.brew.sh/formula/librime). Pass `$(brew --prefix librime)/lib/librime.dylib` and point the engine at your Rime data, such as the shared resources bundled with Squirrel. |

Pass an absolute `LibraryPath` to select the intended build. With an empty path, the loader tries `librime.so.1` and `librime.so` on Linux, `librime.1.dylib` and `librime.dylib` on macOS, or `rime.dll` and `librime.dll` on Windows. The Windows fallback checks the application and system loader locations and absolute `PATH` directories. Other target platforms return `abi.ErrUnsupported` from `abi.Open`.

Add the high-level package to your Go module.

```sh
go get github.com/rimeinn/go-rime/rime
```

## Input

The [`rime`](rime) package manages the engine, sessions and configs. Its returned strings and slices are Go-owned copies. Use a writable user directory. Deploy the workspace once if its schemas have yet to be compiled.

```go
import (
    "fmt"

    "github.com/rimeinn/go-rime/rime"
)

func input(library, shared, user string) error {
    engine, err := rime.Open(rime.Options{
        LibraryPath:   library,
        SharedDataDir: shared,
        UserDataDir:   user,
    })
    if err != nil {
        return err
    }
    defer engine.Close()

    session, err := engine.NewSession()
    if err != nil {
        return err
    }
    defer session.Close()

    for _, key := range "ni" {
        handled, err := session.ProcessKey(key, 0)
        if err != nil {
            return err
        }
        if !handled {
            fmt.Printf("unhandled key %q\n", key)
        }
    }
    ctx, ok, err := session.Context()
    if err != nil || !ok {
        return err
    }
    if len(ctx.Menu.Candidates) > 0 {
        fmt.Println(ctx.Menu.Candidates[0].Text)
        if _, err := session.SelectCandidateOnCurrentPage(0); err != nil {
            return err
        }
    }
    text, ok, err := session.TakeCommit()
    if ok {
        fmt.Println(text)
    }
    return err
}
```

`ProcessKey` takes a Rime/X11 keysym and modifier mask. An unhandled key or absent commit returns `false` with a nil error. Caret and composition offsets are UTF-8 byte offsets. The [console example](examples/console) accepts a schema ID and has a `-deploy` option for a fresh user directory.

## Configuration

`OpenConfig`, `OpenSchemaConfig` and `OpenUserConfig` use librime's respective config sources. `NewConfig` creates an in-memory config. Call `Close` to release each handle. Map and list iteration copy entries and release the native iterator before returning, so the Go iterator can be stopped early.

```go
cfg, err := engine.OpenConfig("default")
if err != nil {
    return err
}
defer cfg.Close()

pageSize, ok, err := cfg.GetInt("menu/page_size")
if err != nil {
    return err
}
if ok {
    fmt.Println(pageSize)
}
```

The [config example](examples/config) reads a string by key. Config handles share the engine lock. Treat changes to an opened config according to the native config source's persistence rules.

## Lifecycle and notifications

`rime.Open` calls native setup and initialization. A process can successfully open one `Engine`. `Close` joins native maintenance, releases sessions and configs, unregisters notifications, finalizes librime and keeps the native library resident for the life of the process. Close methods are idempotent. Applications using both packages must coordinate access to librime's process-wide service.

The wrapper serializes native calls. While asynchronous deployment or synchronization is running, input and config methods may return `ErrMaintenance`. Use `JoinMaintenance` to wait, or `IsMaintaining` to poll. `SyncUserData` invalidates all existing sessions once the native request is made, regardless of whether librime starts the worker.

`Subscribe` returns an event channel, an idempotent cancellation function and an error. The callback copies native strings and queues events without calling application code. Each subscription buffers 32 events and drops new arrivals when full. The engine closes all subscriptions when it closes. Deployment progress and results arrive as `deploy` events.

## ABI coverage

The [`abi`](abi) package exposes the default integer-`Bool` ABI, including 100 slots in the upstream `RimeApi` table and 32 in `RimeLeversApi`. It uses the runtime `data_size` and NULL checks before registering a slot. `Capability` distinguishes missing, NULL, unsupported and available slots. The protobuf slots are present in the upstream table and currently NULL in upstream librime. Builds exporting `rime_get_api_stdbool` alongside the default ABI use the default `rime_get_api` entry point here.

`abi` leaves setup, finalization, resource ownership and synchronization to the caller. Keep `Library` open for every live API function, pointer and resource. Initialize versioned output structs with their `Init` methods and pair native allocations with their documented librime release functions. `StringSlice.String` copies a length-delimited string such as an abbreviated state label.

`Library.RegisterGoModule` supports a process-lifetime Go module with scalar lifecycle callbacks and an optional custom function table. Call `abi.Open`, `api.Setup`, `library.RegisterGoModule`, then `api.Initialize` with the registered name in `Traits.Modules`. Call `api.Finalize` to run module finalizers. Native librime retains registered modules for the life of the process, so `Library.Close` returns `abi.ErrUnsupported` after successful registration. Purego's callback slots are finite and remain allocated after use.

The high-level package covers input and commits, context and status, schemas, candidates, options and properties, configs, deployment and user-data sync. Candidate preview requires the two optional tail slots added after librime 1.17.0. `Engine.Available` checks a C slot by name and optional methods return `rime.ErrUnsupported` when their required slots are missing. APIs that read into a caller-supplied string buffer retry truncation up to a 16 MiB buffer.

The API declarations come from the upstream revision recorded in `abi/internal/apigen`. Regenerate the bindings after updating the ordered declarations. [rime_api.h](https://github.com/rime/librime/blob/bb56876890ba60f446fa2640c244b1794b965eda/src/rime_api.h) · [rime_levers_api.h](https://github.com/rime/librime/blob/bb56876890ba60f446fa2640c244b1794b965eda/src/rime_levers_api.h)

```sh
go generate ./abi
```

## Tests

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go build ./...
go vet ./...
```

Set `RIME_INCLUDE_DIR` to a directory containing the official C headers to run the native layout probe. Use `CC` to select a GCC- or Clang-compatible compiler. Set `GO_RIME_TEST_LIBRARY` to a matching shared library to enable integration tests. Those tests copy the minimal fixture in `rime/testdata/shared` into temporary directories and deploy it at runtime. The C compiler is only used by tests.

```sh
CGO_ENABLED=0 RIME_INCLUDE_DIR=/usr/include GO_RIME_TEST_LIBRARY=librime.so.1 go test -count=1 -v ./...
```

## License

go-rime is licensed under the [MIT License](LICENSE).
