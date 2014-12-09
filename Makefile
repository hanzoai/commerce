pwd				= $(shell pwd)
os				= $(shell uname | tr '[A-Z]' '[a-z]')
platform        = $(os)_amd64
sdk				= go_appengine_sdk_$(platform)-1.9.15
sdk_path        = $(pwd)/.sdk
goroot          = $(sdk_path)/goroot
gopath          = $(sdk_path)/gopath
goroot_pkg_path = $(goroot)/pkg/$(platform)_appengine/
gopath_pkg_path = $(gopath)/pkg/$(platform)_appengine/

deps			= $(shell cat Godeps | cut -d ' ' -f 1)
modules			= crowdstart.io/api \
				  crowdstart.io/checkout \
				  crowdstart.io/platform \
				  crowdstart.io/preorder \
				  crowdstart.io/store

gae_token		= 1/DLPZCHjjCkiegGp0SiIvkWmtZcUNl15JlOg4qB0-1r0MEudVrK5jSpoR30zcRFq6

gae_development = config/development/app.yaml \
				  config/development/dispatch.yaml \
				  api/app.dev.yaml \
				  checkout/app.dev.yaml \
				  platform/app.dev.yaml \
				  preorder/app.dev.yaml \
				  store/app.dev.yaml

gae_staging  = config/staging \
			   api/app.staging.yaml \
			   checkout/app.staging.yaml \
			   platform/app.staging.yaml \
			   preorder/app.staging.yaml \
			   store/app.staging.yaml

gae_skully  = config/skully \
			  api/app.skully.yaml \
			  checkout/app.skully.yaml \
			  preorder/app.skully.yaml \
			  store/app.skully.yaml

gae_production  = config/production \
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

# replacement file watcher for the dev appengine
mtime_file_watcher = https://gist.githubusercontent.com/zeekay/d92deea5091849d79782/raw/a2f43b902afef21a2a53f4ca529975a28b20d943/mtime_file_watcher.py

# static assets, requisite javascript from assets -> static
bebop = node_modules/.bin/bebop

requisite	   = node_modules/.bin/requisite -s -g
requisite_opts = assets/js/store/store.coffee \
				 assets/js/preorder/preorder.coffee \
				 assets/js/checkout/checkout.coffee \
				 -o static/js/store.js \
				 -o static/js/preorder.js \
				 -o static/js/checkout.js
requisite_opts_min = -m --strip-debug


stylus		   = node_modules/.bin/stylus
stylus_opts    = assets/css/preorder/preorder.styl \
				 assets/css/store/store.styl \
				 assets/css/checkout/checkout.styl \
				 -o static/css -u autoprefixer-stylus
stylus_opts_min = -u csso-stylus -c

# find command differs between bsd/linux thus the two versions
ifeq ($(os), "linux")
	packages	 = $(shell find . -maxdepth 4 -mindepth 2 -name '*.go' -printf '%h\n' | sort -u | sed -e 's/.\//crowdstart.io\//')
	test_modules = $(shell find . -maxdepth 4 -mindepth 3 -name '*_test.go' -printf '%h\n' | sort -u | sed -e 's/.\//crowdstart.io\//')
else
	packages	 = $(shell find . -maxdepth 4 -mindepth 2 -name '*.go' -print0 | xargs -0 -n1 dirname | sort --unique | sed -e 's/.\//crowdstart.io\//')
	test_modules = $(shell find . -maxdepth 4 -mindepth 2 -name '*_test.go' -print0 | xargs -0 -n1 dirname | sort --unique | sed -e 's/.\//crowdstart.io\//')
endif

ifeq ($(v), 1)
	verbose = -v
else
	verbose =
endif

export GOROOT  := $(goroot)
export GOPATH  := $(gopath)

all: deps assets test install

assets: deps-assets compile-css compile-js

assets-min: deps-assets compile-css-min compile-js-min

compile-js:
	$(requisite) $(requisite_opts)

compile-js-min:
	$(requisite) $(requisite_opts) $(requisite_opts_min)

compile-css:
	$(stylus) $(stylus_opts) --sourcemap --sourcemap-inline

compile-css-min:
	$(stylus) $(stylus_opts) $(stylus_opts_min)

live-reload: assets
	$(bebop)

build: deps
	goapp build $(modules)

node_modules/.bin/bebop:
	npm install bebop@latest

node_modules/.bin/requisite:
	npm install requisite@latest

node_modules/.bin/stylus:
	npm install stylus@latest

deps-assets: node_modules/.bin/bebop node_modules/.bin/requisite node_modules/.bin/stylus
	npm install

deps-go: .sdk
	gpm install || curl -s https://raw.githubusercontent.com/pote/gpm/v1.3.1/bin/gpm | bash

deps: deps-go deps-assets

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
	# curl  $(mtime_file_watcher) > $(sdk_path)/google/appengine/tools/devappserver2/mtime_file_watcher.py && \
	# pip install watchdog

serve: assets
	$(sdk_path)/dev_appserver.py --datastore_path=~/.gae_datastore.bin $(gae_development)

serve-clear-datastore: assets
	$(sdk_path)/dev_appserver.py --datastore_path=~/.gae_datastore.bin --clear_datastore=true $(gae_development)

serve-no-restart: assets
	$(sdk_path)/dev_appserver.py --datastore_path=~/.gae_datastore.bin --automatic_restart=false $(gae_development)

serve-public: assets
	$(sdk_path)/dev_appserver.py --host=0.0.0.0 --datastore_path=~/.gae_datastore.bin $(gae_development)

tools:
	goapp get $(tools) && \
	goapp install $(tools) && \
	gocode set lib-path "$(gopath_pkg_path):$(goroot_pkg_path)"

test:
	goapp test -timeout 60s $(test_modules) $(verbose)

bench: build
	goapp test -timeout 60s $(test_modules) $(verbose) --bench=.

deploy: test
	go run scripts/deploy.go

deploy-appengine: assets-min
	for module in $(gae_production); do \
		$(sdk_path)/appcfg.py --skip_sdk_update_check rollback $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check update $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check set_default_version $$module; \
	done; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_dispatch config/production

deploy-appengine-staging: assets-min
	for module in $(gae_staging); do \
		$(sdk_path)/appcfg.py --skip_sdk_update_check rollback $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check update $$module; \
	done; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_dispatch config/staging

deploy-appengine-skully: assets-min
	for module in $(gae_skully); do \
		$(sdk_path)/appcfg.py --skip_sdk_update_check rollback $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check update $$module; \
	done; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_dispatch config/skully

deploy-appengine-ci: assets-minified
	for module in $(gae_production); do \
		$(sdk_path)/appcfg.py --skip_sdk_update_check rollback $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check update $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check set_default_version $$module; \
	done; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_dispatch config/production

# Usage: make datastore-export kind=user
datastore-export:
	mkdir -p _export/ && \
	bulkloader.py --download \
				  --url http://static.skullysystems.com/_ah/remote_api \
				  --config_file config/skully/bulkloader.yaml \
				  --db_filename /tmp/bulkloader-$$kind.db \
				  --log_file /tmp/bulkloader-$$kind.log \
				  --result_db_filename /tmp/bulkloader-result-$$kind.db \
				  --kind $$kind \
				  --filename _export/$$kind.csv && \
	rm -rf /tmp/bulkloader-$$kind.db /tmp/bulkloader-$$kind.log /tmp/bulkloader-result-$$kind.db
