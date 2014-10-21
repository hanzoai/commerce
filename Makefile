sdk = go_appengine_sdk_darwin_amd64-1.9.13

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
	mv go_appengine sdk && \
	rm $(sdk).zip && \
	mkdir -p sdk/gopath/src/verus.io && \
	ln -s $(pwd)/src $(pwd)/sdk/gopath/src/verus.io/crowdstart

serve:
	goapp serve verus.io/crowdstart

tools:
	goapp get $(tools) && goapp install $(tools)

test:
	goapp test verus.io/crowdstart/test

bench:
	goapp test verus.io/crowdstart/test --bench=.

.PHONY: all build deps test serve tools
