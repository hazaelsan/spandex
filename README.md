spandex
=======

Converts text expansion snippets between different text expander formats.

Supported formats
-----------------

* TextExpander
* AutoKey

Requirements
------------

* [go-plist](https://github.com/DHowett/go-plist)
* [glog](https://github.com/golang/glog)

Running it
----------

```shell
$ $GOBIN/spandex -logtostderr -source TextExpander -dest AutoKey
```
