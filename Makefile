platform        = $(shell uname | tr '[A-Z]' '[a-z]')_amd64
sdk 	        = go_appengine_sdk_$(platform)-1.9.13
sdk_path        = $(shell pwd)/sdk
goroot_pkg_path = $(sdk_path)/goroot/pkg/$(platform)_appengine/
gopath_pkg_path = $(sdk_path)/gopath/pkg/$(platform)_appengine/

tools = github.com/nsf/gocode \
        code.google.com/p/go.tools/cmd/goimports \
        code.google.com/p/rog-go/exp/cmd/godef \
        code.google.com/p/go.tools/cmd/oracle \
        code.google.com/p/go.tools/cmd/gorename \
        github.com/golang/lint/golint \
        github.com/kisielk/errcheck \
        github.com/jstemmer/gotags

all: deps test

build: sdk
	goapp build verus.io/crowdstart

deps: sdk
	gpm install || curl -s https://raw.githubusercontent.com/pote/gpm/v1.3.1/bin/gpm | bash

sdk:
	wget https://storage.googleapis.com/appengine-sdks/featured/$(sdk).zip && \
	unzip $(sdk).zip && \
	mv go_appengine $(sdk_path) && \
	rm $(sdk).zip && \
	mkdir -p $(sdk_path)/gopath/src/verus.io && \
	ln -s $(pwd)/src $(sdk_path)/gopath/src/verus.io/crowdstart && \

serve:
	goapp serve verus.io/crowdstart

tools:
	goapp get $(tools) && \
	goapp install $(tools) && \
	gocode set lib-path "$(gopath_pkg_path):$(goroot_pkg_path)"

test:
	goapp test verus.io/crowdstart/test

bench:
	goapp test verus.io/crowdstart/test --bench=.

deploy:
	goapp deploy verus.io/crowdstart

.PHONY: all build deploy deps test serve tools
