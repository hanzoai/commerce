pwd 			= $(shell pwd)
os		        = $(shell uname | tr '[A-Z]' '[a-z]')
platform        = $(os)_amd64
sdk 	        = go_appengine_sdk_$(platform)-1.9.15
sdk_path        = $(pwd)/.sdk
goroot          = $(sdk_path)/goroot
gopath          = $(sdk_path)/gopath
goroot_pkg_path = $(goroot)/pkg/$(platform)_appengine/
gopath_pkg_path = $(gopath)/pkg/$(platform)_appengine/

deps 		    = $(shell cat Godeps | cut -d ' ' -f 1)
modules 	    = crowdstart.io/api \
				  crowdstart.io/checkout \
				  crowdstart.io/platform \
				  crowdstart.io/preorder \
				  crowdstart.io/store

gae_token 	    = 1/DLPZCHjjCkiegGp0SiIvkWmtZcUNl15JlOg4qB0-1r0MEudVrK5jSpoR30zcRFq6

gae_development = config/dev/dispatch.yaml \
				  api/app.dev.yaml \
				  checkout/app.dev.yaml \
				  config/dev/app.yaml \
				  platform/app.dev.yaml \
				  preorder/app.dev.yaml \
				  store/app.dev.yaml

gae_production  = config/prod \
				  api \
				  checkout \
			      platform \
				  preorder \
				  store

tools = github.com/nsf/gocode \
        code.google.com/p/go.tools/cmd/goimports \
        code.google.com/p/rog-go/exp/cmd/godef \
        code.google.com/p/go.tools/cmd/oracle \
        code.google.com/p/go.tools/cmd/gorename \
        github.com/golang/lint/golint \
        github.com/kisielk/errcheck \
        github.com/jstemmer/gotags

# find command differs between bsd/linux thus the two versions
ifeq ($(os), "linux")
	packages 	 = $(shell find . -maxdepth 3 -mindepth 2 -name '*.go' -printf '%h\n' | sort -u | sed -e 's/.\//crowdstart.io\//')
	test_modules = $(shell find . -maxdepth 3 -mindepth 3 -name '*_test.go' -printf '%h\n' | sort -u | sed -e 's/.\//crowdstart.io\//')
else
	packages 	 = $(shell find . -maxdepth 3 -mindepth 2 -name '*.go' -print0 | xargs -0 -n1 dirname | sort --unique | sed -e 's/.\//crowdstart.io\//')
	test_modules = $(shell find . -maxdepth 3 -mindepth 2 -name '*_test.go' -print0 | xargs -0 -n1 dirname | sort --unique | sed -e 's/.\//crowdstart.io\//')
endif

export GOROOT  := $(goroot)
export GOPATH  := $(gopath)

all: deps assets test

assets: deps-js
	node_modules/.bin/requisite assets/js/store/store.coffee -g -o static/js/store.js && \
	node_modules/.bin/requisite assets/js/checkout/checkout.coffee -g -o static/js/checkout.js && \
	node_modules/.bin/requisite assets/js/preorder/preorder.coffee -g -o static/js/preorder.js

assets-watch: deps-js
	node_modules/.bin/requisite assets/js/preorder/preorder.coffee -w -g -o static/js/preorder.js

build: deps
	goapp build $(modules)

node_modules/.bin/requisite:
	npm install requisite

deps-js: node_modules/.bin/requisite
	npm install

deps-go: .sdk
	gpm install || curl -s https://raw.githubusercontent.com/pote/gpm/v1.3.1/bin/gpm | bash

deps: deps-go deps-js

install: install-deps
	goapp install $(modules) $(packages)

install-deps:
	goapp install $(deps)

.sdk:
	wget https://storage.googleapis.com/appengine-sdks/featured/$(sdk).zip && \
	unzip $(sdk).zip && \
	mv go_appengine $(sdk_path) && \
	rm $(sdk).zip && \
	mkdir -p $(sdk_path)/gopath/src && \
	mkdir -p $(sdk_path)/gopath/bin && \
	ln -s $(shell pwd) $(sdk_path)/gopath/src/crowdstart.io && \
	echo '#!/usr/bin/env bash\ngoapp $$@' > $(sdk_path)/gopath/bin/go && \
	chmod +x $(sdk_path)/gopath/bin/go

serve:
	$(sdk_path)/dev_appserver.py --datastore_path=~/.gae_datastore.bin $(gae_development)

serve-clear-datastore:
	$(sdk_path)/dev_appserver.py --datastore_path=~/.gae_datastore.bin --clear_datastore=true $(gae_development)

tools:
	goapp get $(tools) && \
	goapp install $(tools) && \
	gocode set lib-path "$(gopath_pkg_path):$(goroot_pkg_path)"

test:
	goapp test -timeout 60s $(test_modules)

bench: build
	goapp test -timeout 60s $(test_modules) --bench=.

deploy: test
	go run deploy.go

deploy-appengine: assets
	for module in $(gae_production); do \
		$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) rollback $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) update $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) set_default_version $$module; \
	done && \
	$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) update_dispatch config/prod

.PHONY: all assets build deploy deps deps-js deps-go serve test tools
