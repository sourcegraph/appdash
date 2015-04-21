install: warning
	go get -t -v ./...

dist: warning gen-dist
	go install -tags 'dist' ./...

gen-dist: warning go-bindata
	go-bindata -ignore='data.*\.go' -pkg=tmpl -prefix="traceapp/tmpl" -o traceapp/tmpl/data.go traceapp/tmpl/...

gen-dev: warning go-bindata
	go-bindata -dev -ignore='data.*\.go' -pkg=tmpl -prefix="traceapp/tmpl" -o traceapp/tmpl/data.go traceapp/tmpl/...

go-bindata:
	go get github.com/jteeuwen/go-bindata/go-bindata

warning:
	# Warning! You are using the outdated Makefile build process for Appdash!
	#
	# Please upgrade to the go:generate build process, as this process will be
	# removed in the future!
	#
