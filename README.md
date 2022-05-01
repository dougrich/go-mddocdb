# go-mddocdb

[![build](https://img.shields.io/github/workflow/status/dougrich/go-mddocdb/build?style=flat-square)](https://github.com/dougrich/go-mddocdb/actions/workflows/build.yml)

go-mddocdb takes an interface to a file system and returns an http.handler. When a request comes in (e.g. `/example`) it'll try to read the appropriate file (`/example.md`) from the file system - if it finds it, it'll turn it from markdown into html. It does some in memory caching.

```
mddocdb.NewHandler(
    fs, // the filesystem to read from, which should include a 'get' that returns back a reader
    "", // the basepath to ignore
    &mddocdb.Options{
        CacheDurationSeconds: 300, // how long to keep the cache around for
        Template: ..., // the template to use for the html page
    },
)
```