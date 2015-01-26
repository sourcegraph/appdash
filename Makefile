default: install

go-bindata:
	go get github.com/jteeuwen/go-bindata/go-bindata

gen-dist: go-bindata
	go-bindata -ignore='data.go' -pkg=tmpl -prefix="traceapp/tmpl" -o traceapp/tmpl/data.go traceapp/tmpl/...

gen-dev: go-bindata
	go-bindata -debug -ignore='data.go' -pkg=tmpl -prefix="traceapp/tmpl" -o traceapp/tmpl/data.go traceapp/tmpl/...

install: gen-dist
	go install ./...
