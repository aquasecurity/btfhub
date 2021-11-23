## Example of how to use BTFHUB

* running example in a recent kernel:

```
$ sudo ./example-static
$ sudo ./example-c-static
```

* running example in an older kernel, together with external BTF file from BTFHUB:

```
$ sudo EXAMPLE_BTF_FILE=5.8.0-63-generic.btf ./example-static
$ sudo EXAMPLE_BTF_FILE=5.8.0-63-generic.btf ./example-c-static
```
