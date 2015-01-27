install:
	go get -t -v ./...

dist: gen-dist
	go install -tags 'dist' ./...

gen-dist: go-bindata
	go-bindata -ignore='data.go' -pkg=tmpl -prefix="traceapp/tmpl" -o traceapp/tmpl/data.go traceapp/tmpl/...

gen-dev: go-bindata
	go-bindata -dev -ignore='data.*\.go' -pkg=tmpl -prefix="traceapp/tmpl" -o traceapp/tmpl/data.go traceapp/tmpl/...

go-bindata:
	go get github.com/jteeuwen/go-bindata/go-bindata
