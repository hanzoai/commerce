pwd 			= $(shell pwd)
platform        = $(shell uname | tr '[A-Z]' '[a-z]')_amd64
sdk 	        = go_appengine_sdk_$(platform)-1.9.14
sdk_path        = $(pwd)/.sdk
goroot          = $(sdk_path)/goroot
gopath          = $(sdk_path)/gopath
goroot_pkg_path = $(goroot)/pkg/$(platform)_appengine/
gopath_pkg_path = $(gopath)/pkg/$(platform)_appengine/

deps 		    = $(shell cat Godeps | cut -d ' ' -f 1)
modules 	    = crowdstart.io/admin \
				  crowdstart.io/api \
				  crowdstart.io/checkout \
				  crowdstart.io/store

packages 		= crowdstart.io/cardconnect \
				  crowdstart.io/datastore \
				  crowdstart.io/middleware \
				  crowdstart.io/models \
				  crowdstart.io/sessions \
				  crowdstart.io/util \

test_modules    = crowdstart.io/admin/test \
				  crowdstart.io/api/test \
				  crowdstart.io/checkout/test \
				  crowdstart.io/store/test

gae_token 	    = 1/DLPZCHjjCkiegGp0SiIvkWmtZcUNl15JlOg4qB0-1r0MEudVrK5jSpoR30zcRFq6
gae_yaml  	    = dispatch.yaml \
				  app.yaml \
				  admin/app.yaml \
				  api/app.yaml \
				  store/app.yaml \
				  checkout/app.yaml

tools = github.com/nsf/gocode \
        code.google.com/p/go.tools/cmd/goimports \
        code.google.com/p/rog-go/exp/cmd/godef \
        code.google.com/p/go.tools/cmd/oracle \
        code.google.com/p/go.tools/cmd/gorename \
        github.com/golang/lint/golint \
        github.com/kisielk/errcheck \
        github.com/jstemmer/gotags

export GOROOT  := $(goroot)
export GOPATH  := $(gopath)

all: deps test

assets:
	requisite assets/js/crowdstart.coffee -o static/js/crowdstart.js

assets-watch:
	requisite -w assets/js/crowdstart.coffee -o static/js/crowdstart.js

build: deps
	goapp build $(modules)

deps: .sdk assets
	(gpm install || curl -s https://raw.githubusercontent.com/pote/gpm/v1.3.1/bin/gpm | bash) && \
	(hash requisite 2>/dev/null || npm install -g requisite)

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
	ln -s $(shell pwd) $(sdk_path)/gopath/src/crowdstart.io

serve:
	$(sdk_path)/dev_appserver.py $(gae_yaml)

tools:
	goapp get $(tools) && \
	goapp install $(tools) && \
	gocode set lib-path "$(gopath_pkg_path):$(goroot_pkg_path)"

test: build
	goapp test $(test_modules)

bench: build
	goapp test $(test_modules) --bench=.

deploy:
	goapp run deploy.go

deploy-appengine:
	$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) rollback app.yaml && \
	$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) rollback api/app.yaml && \
	$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) rollback checkout/app.yaml && \
	$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) rollback store/app.yaml && \
	$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) update app.yaml && \
	$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) update admin/app.yaml && \
	$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) update api/app.yaml && \
	$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) update checkout/app.yaml && \
	$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) update store/app.yaml && \
	$(sdk_path)/appcfg.py --skip_sdk_update_check --oauth2_refresh_token=$(gae_token) update_dispatch .

.PHONY: all assets build deploy deps test serve tools
