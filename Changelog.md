# 0.0.5 / 2024-12-12

- Add support `fsys.Cache = cache.Memory()` and exposes `cache/` package.
  Providing a cache greatly reduces the number of times that generators are called.
- Support an optional `fsys.Root = dir` to work better with real filesystems

# 0.0.4 / 2024-12-12

- bump virt
- add sanity test

# 0.0.3 / 2024-03-23

- fix error prefix, add fs.Sub test

# 0.0.2 / 2024-03-21

- simplify internals, reduce scope
- tree improvements around dynamically discovering nodes
- switch to `*virt.File`
- add support for multiple dir generators sharing common directories
- support root generators.

# 0.0.1 / 2024-03-17

- update readme with example
- initial commit
