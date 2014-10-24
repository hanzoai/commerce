platform        = $(shell uname | tr '[A-Z]' '[a-z]')_amd64
sdk 	        = go_appengine_sdk_$(platform)-1.9.13
sdk_path        = $(shell pwd)/.sdk
goroot          = $(sdk_path)/goroot
gopath          = $(sdk_path)/gopath
goroot_pkg_path = $(goroot)/pkg/$(platform)_appengine/
gopath_pkg_path = $(gopath)/pkg/$(platform)_appengine/

tools = github.com/nsf/gocode \
        code.google.com/p/go.tools/cmd/goimports \
        code.google.com/p/rog-go/exp/cmd/godef \
        code.google.com/p/go.tools/cmd/oracle \
        code.google.com/p/go.tools/cmd/gorename \
        github.com/golang/lint/golint \
        github.com/kisielk/errcheck \
        github.com/jstemmer/gotags

export GOROOT:= $(goroot)
export GOPATH:= $(gopath)

all: deps test

build: .sdk
	goapp build crowdstart.io/api crowdstart.io/checkout crowdstart.io/store

deps: .sdk
	gpm install || curl -s https://raw.githubusercontent.com/pote/gpm/v1.3.1/bin/gpm | bash

.sdk:
	wget https://storage.googleapis.com/appengine-sdks/featured/$(sdk).zip && \
	unzip $(sdk).zip && \
	mv go_appengine $(sdk_path) && \
	rm $(sdk).zip && \
	mkdir -p $(sdk_path)/gopath/src/crowdstart.io && \
	ln -s $(pwd)/src $(sdk_path)/gopath/src/crowdstart.io/admin && \
	ln -s $(pwd)/src $(sdk_path)/gopath/src/crowdstart.io/api && \
	ln -s $(pwd)/src $(sdk_path)/gopath/src/crowdstart.io/checkout && \
	ln -s $(pwd)/src $(sdk_path)/gopath/src/crowdstart.io/middleware && \
	ln -s $(pwd)/src $(sdk_path)/gopath/src/crowdstart.io/store

serve:
	$(sdk_path)/dev_appserver.py dispatch.yaml app.yaml api/app.yaml store/app.yaml checkout/app.yaml

tools:
	goapp get $(tools) && \
	goapp install $(tools) && \
	gocode set lib-path "$(gopath_pkg_path):$(goroot_pkg_path)"

test:
	goapp test crowdstart.io/api/test crowdstart.io/checkout/test crowdstart.io/store/test

bench:
	goapp test crowdstart.io/api/test crowdstart.io/checkout/test crowdstart.io/store/test --bench=.

deploy:
	$(sdk_path)/appcfg.py update app.yaml api/app.yaml checkout/app.yaml store/app.yaml && \
	$(sdk_path)/appcfg.py update_dispatch .

.PHONY: all build deploy deps test serve tools
