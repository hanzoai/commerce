pwd				= $(shell pwd)
os				= $(shell uname | tr '[A-Z]' '[a-z]')
platform		= $(os)_amd64
sdk				= go_appengine_sdk_$(platform)-1.9.18
sdk_path		= $(pwd)/.sdk
goroot			= $(sdk_path)/goroot
gopath			= $(sdk_path)/gopath
goroot_pkg_path = $(goroot)/pkg/$(platform)_appengine/
gopath_pkg_path = $(gopath)/pkg/$(platform)_appengine/
current_date	= $(shell date +"%Y-%m-%d")

goapp			= $(sdk_path)/goapp
gpm				= GOPATH=$(gopath) PATH=$(sdk_path):$$PATH $(sdk_path)/gpm
ginkgo			= GOPATH=$(gopath) PATH=$(sdk_path):$$PATH $(gopath)/bin/ginkgo

deps	= $(shell cat Godeps | cut -d ' ' -f 1)
modules	= crowdstart.io/api \
		  crowdstart.io/checkout \
		  crowdstart.io/platform \
		  crowdstart.io/preorder \
		  crowdstart.io/store

gae_token = 1/DLPZCHjjCkiegGp0SiIvkWmtZcUNl15JlOg4qB0-1r0MEudVrK5jSpoR30zcRFq6

gae_development = config/development/app.yaml \
				  config/development/dispatch.yaml \
				  api/app.dev.yaml \
				  checkout/app.dev.yaml \
				  platform/app.dev.yaml \
				  preorder/app.dev.yaml \
				  store/app.dev.yaml

gae_sandbox = config/sandbox \
			  api/app.staging.yaml

gae_staging = config/staging \
			  api/app.staging.yaml \
			  checkout/app.staging.yaml \
			  platform/app.staging.yaml \
			  preorder/app.staging.yaml \
			  store/app.staging.yaml

gae_skully = config/skully \
			 api/app.skully.yaml \
			 checkout/app.skully.yaml \
			 platform/app.skully.yaml \
			 preorder/app.skully.yaml \
			 store/app.skully.yaml

gae_production = config/production \
				 api \
				 checkout \
				 platform

tools = github.com/nsf/gocode \
		code.google.com/p/go.tools/cmd/goimports \
		code.google.com/p/rog-go/exp/cmd/godef \
		code.google.com/p/go.tools/cmd/oracle \
		golang.org/x/tools/cmd/gorename \
		github.com/golang/lint/golint \
		github.com/kisielk/errcheck \
		github.com/jstemmer/gotags

# Various patches for SDK
mtime_file_watcher = https://gist.githubusercontent.com/zeekay/5eba991c39426ca42cbb/raw/235f107b7ed081719103a4259dddd0e568d12480/mtime_file_watcher.py

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

stylus		= node_modules/.bin/stylus
stylus_opts = assets/css/preorder/preorder.styl \
			  assets/css/store/store.styl \
			  assets/css/theme/theme.styl \
			  assets/css/checkout/checkout.styl \
			  -o static/css
stylus_opts_min = -u csso-stylus -c

autoprefixer = node_modules/.bin/autoprefixer
autoprefixer_opts = -b 'ie > 8, firefox > 24, chrome > 30, safari > 6, opera > 17, ios > 6, android > 4' \
					static/css/checkout.css \
					static/css/preorder.css \
					static/css/store.css \
					static/css/theme.css

sdk_install = wget https://storage.googleapis.com/appengine-sdks/featured/$(sdk).zip && \
			  unzip $(sdk).zip && \
			  mv go_appengine $(sdk_path) && \
			  rm $(sdk).zip && \
			  mkdir -p $(sdk_path)/gopath/src && \
			  mkdir -p $(sdk_path)/gopath/bin && \
			  ln -s $(shell pwd) $(sdk_path)/gopath/src/crowdstart.io

dev_appserver = $(sdk_path)/dev_appserver.py --skip_sdk_update_check \
											 --datastore_path=~/.gae_datastore.bin \
											 --dev_appserver_log_level=error

sdk_install_extra = rm -rf $(sdk_path)/demos

# find command differs between bsd/linux thus the two versions
ifeq ($(os), linux)
	packages = $(shell find . -maxdepth 4 -mindepth 2 -name '*.go' \
			   				  -not -path "./.sdk/*" \
			   				  -not -path "./test/*" \
			   				  -not -path "./assets/*" \
			   				  -not -path "./static/*" \
			   				  -not -path "./node_modules/*" \
			   				  -printf '%h\n' | sort -u | sed -e 's/.\//crowdstart.io\//')
else
	packages = $(shell find . -maxdepth 4 -mindepth 2 -name '*.go' \
			   				  -not -path "./.sdk/*" \
			   				  -not -path "./test/*" \
			   				  -not -path "./assets/*" \
			   				  -not -path "./static/*" \
			   				  -not -path "./node_modules/*" \
			   				  -print0 | xargs -0 -n1 dirname | sort --unique | sed -e 's/.\//crowdstart.io\//')
	sdk_install_extra := $(sdk_install_extra) && \
						 curl $(mtime_file_watcher) > $(sdk_path)/google/appengine/tools/devappserver2/mtime_file_watcher.py && \
						 pip install macfsevents --upgrade
endif

# set v=1 to enable verbose mode
ifeq ($(v), 1)
	test_verbose = -v=true -- -test.v=true
else
	test_verbose =
endif

# set production=1 to set datastore export/import target to use production
ifeq ($(production), 1)
	datastore_app_id = crowdstart-us
else ifeq ($(skully), 1)
	datastore_app_id = crowdstart-skully
else
	datastore_app_id = crowdstart-staging
endif

datastore_admin_url = https://datastore-admin-dot-$(datastore_app_id).appspot.com/_ah/remote_api

test_focus := $(focus)
ifdef test_focus
	test_focus=--focus=$(focus)
endif

export GOROOT := $(goroot)
export GOPATH := $(gopath)

all: deps test install

# ASSETS
assets: deps-assets compile-css compile-js

assets-min: deps-assets compile-css-min compile-js-min

compile-js:
	$(requisite) $(requisite_opts)

compile-js-min:
	$(requisite) $(requisite_opts) $(requisite_opts_min)

compile-css:
	$(stylus) $(stylus_opts) -u autoprefixer-stylus --sourcemap --sourcemap-inline

compile-css-min:
	$(stylus) $(stylus_opts) $(stylus_opts_min) && $(autoprefixer) $(autoprefixer_opts)

# BUILD
build: deps assets
	$(goapp) build $(modules)

# DEPS
deps: deps-assets deps-go

# DEPS JS/CSS
deps-assets:
	npm install

# DEPS GO
deps-go: .sdk .sdk/go .sdk/gpm .sdk/gopath/bin/ginkgo
	$(gpm) install

.sdk:
	$(sdk_install) && $(sdk_install_extra)

.sdk/go:
	echo '#!/usr/bin/env bash' > $(sdk_path)/go && \
	echo '$(sdk_path)/goapp $$@' >> $(sdk_path)/go && \
	chmod +x $(sdk_path)/go

.sdk/gpm:
	curl -s https://raw.githubusercontent.com/pote/gpm/v1.3.2/bin/gpm > .sdk/gpm && \
	chmod +x .sdk/gpm

.sdk/gopath/bin/ginkgo:
	$(gpm) install && $(goapp) install github.com/onsi/ginkgo/ginkgo

# INSTALL
install: install-deps
	$(goapp) install $(modules) $(packages)

install-deps:
	$(goapp) install $(deps)

# DEV SERVER
serve: assets
	$(dev_appserver) $(gae_development)

serve-clear-datastore: assets
	$(dev_appserver) --clear_datastore=true $(gae_development)

serve-public: assets
	$(dev_appserver) --host=0.0.0.0 $(gae_development)

# LIVE RELOAD SERVER
live-reload: assets
	$(bebop)

# GOLANG TOOLS
tools:
	$(goapp) get $(tools) && \
	$(goapp) install $(tools) && \
	$(gopath)/bin/gocode set lib-path "$(gopath_pkg_path):$(goroot_pkg_path)"

# TEST/ BENCH
test:
	@$(ginkgo) -r=true --randomizeAllSpecs -p=true -progress=true -skipMeasurements=true -skipPackage=integration $(test_focus) $(test_verbose)

test-integration:
	@$(ginkgo) -r=true --randomizeAllSpecs -p=true -progress=true -skipMeasurements=true -focus=integration $(test_verbose)

test-watch:
	@$(ginkgo) watch -r=true -p=true -progress=true -skipMeasurements=true $(test_focus) $(test_verbose)

bench:
	@$(ginkgo) -r=true --randomizeAllSpecs -p=true -progress=true -skipPackage=integration $(test_focus) $(test_verbose)

test-ci:
	$(ginkgo) -r=true --randomizeAllSpecs --randomizeSuites --failOnPending --cover --trace --compilers=2 -v=true -- -test.v=true

# DEPLOY
deploy: test
	go run scripts/deploy.go

deploy-production: assets-min
	for module in $(gae_production); do \
		$(sdk_path)/appcfg.py --skip_sdk_update_check rollback $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check update $$module; \
	done; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_indexes config/production; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_dispatch config/production

deploy-staging: assets
	for module in $(gae_staging); do \
		$(sdk_path)/appcfg.py --skip_sdk_update_check rollback $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check update $$module; \
	done; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_indexes config/staging; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_dispatch config/staging

deploy-skully: assets-min
	for module in $(gae_skully); do \
		$(sdk_path)/appcfg.py --skip_sdk_update_check rollback $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check update $$module; \
	done; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_indexes config/skully; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_dispatch config/skully

deploy-appengine-ci: assets-minified
	for module in $(gae_production); do \
		$(sdk_path)/appcfg.py --skip_sdk_update_check rollback $$module; \
		$(sdk_path)/appcfg.py --skip_sdk_update_check update $$module; \
	done; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_indexes config/production; \
	$(sdk_path)/appcfg.py --skip_sdk_update_check update_dispatch config/production

# EXPORT / Usage: make datastore-export kind=user
datastore-export:
	@mkdir -p _export/ && \
	bulkloader.py --download \
				  --bandwidth_limit 1000000000 \
				  --rps_limit 10000 \
				  --batch_size 250 \
				  --http_limit 200 \
				  --url $(datastore_admin_url) \
				  --config_file util/bulkloader/bulkloader-export.yaml \
				  --db_filename /tmp/bulkloader-$$kind.db \
				  --log_file /tmp/bulkloader-$$kind.log \
				  --result_db_filename /tmp/bulkloader-result-$$kind.db \
				  --kind $$kind \
				  --filename _export/$$kind-$(datastore_app_id)-$(current_date).csv && \
	rm -rf /tmp/bulkloader-$$kind.db \
		   /tmp/bulkloader-$$kind.log \
		   /tmp/bulkloader-result-$$kind.db

# IMPORT / Usage: make datastore-import kind=user file=user.csv
datastore-import:
	@appcfg.py upload_data --bandwidth_limit 1000000000 \
						  --rps_limit 10000 \
						  --batch_size 250 \
						  --http_limit 200 \
						  --url $(datastore_admin_url) \
						  --config_file util/bulkloader/bulkloader-import.yaml \
						  --kind $$kind \
						  --filename $$file \
						  --log_file /tmp/bulkloader-upload-$$kind.log && \
	rm -rf /tmp/bulkloader-upload-$$kind.log

# Generate config for use with datastore-export target
datastore-config:
	@bulkloader.py --create_config \
				  --url=$(datastore_admin_url) \
				  --filename=bulkloader.yaml

.PHONY: all bench build compile-js compile-js-min compile-css compile-css-min \
	datastore-import datastore-export datastore-config deploy deploy-staging \
	deploy-skully deploy-production deps deps-assets deps-go live-reload \
	serve serve-clear-datastore serve-public test test-integration test-watch \
	tools
