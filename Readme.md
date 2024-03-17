# genfs

[![Go Reference](https://pkg.go.dev/badge/github.com/matthewmueller/genfs.svg)](https://pkg.go.dev/github.com/matthewmueller/genfs)

genfs is a feature-rich generator system that behaves like a filesystem. This package formed the foundation for [bud](https://github.com/livebud/bud).

## Install

```sh
go get github.com/matthewmueller/genfs
```

## Usage

```go
fsys := genfs.New()
fsys.GenerateFile("a.txt", func(fsys genfs.FS, file *genfs.File) error {
  file.WriteString("a")
  return nil
})
code, _ := fs.ReadFile(fsys, "a.txt")
fmt.Println(string(code))
// Output: a
```

## Contributors

- Matt Mueller ([@mattmueller](https://twitter.com/mattmueller))

## License

MIT
