pwd				= $(shell pwd)
os				= $(shell uname | tr '[A-Z]' '[a-z]')
platform		= $(os)_amd64
sdk				= go_appengine_sdk_$(platform)-1.9.40
sdk_path		= $(pwd)/.sdk
goroot			= $(sdk_path)/goroot
gopath			= $(sdk_path)/gopath
goroot_pkg_path = $(goroot)/pkg/$(platform)_appengine/
gopath_pkg_path = $(gopath)/pkg/$(platform)_appengine/
current_date	= $(shell date +"%Y-%m-%d")

goapp			= $(sdk_path)/goapp
ginkgo			= GOPATH=$(gopath) PATH=$(sdk_path):$$PATH $(gopath)/bin/ginkgo
glide			= $(gopath)/bin/glide
appcfg.py 		= $(sdk_path)/appcfg.py --skip_sdk_update_check
bulkloader.py   = $(sdk_path)/bulkloader.py

deps	= $(shell cat Godeps | cut -d ' ' -f 1)
modules	= hanzo.io/analytics \
		  hanzo.io/api

gae_development = config/development/app.yaml \
				  config/development/dispatch.yaml \
				  analytics/app.dev.yaml \
				  api/app.dev.yaml

gae_sandbox = config/sandbox \
			  analytics/app.sandbox.yaml \
			  api/app.sandbox.yaml

gae_staging = config/staging \
			  analytics/app.staging.yaml \
			  api/app.staging.yaml

gae_production = config/production \
				 analytics \
				 api

tools = github.com/nsf/gocode \
        golang.org/x/tools/cmd/guru \
        golang.org/x/tools/cmd/gorename \
        github.com/golang/lint/golint \
        github.com/rogpeppe/godef \
        github.com/kisielk/errcheck \
        github.com/jstemmer/gotags \
        github.com/klauspost/asmfmt/cmd/asmfmt \
        github.com/fatih/motion \
        github.com/zmb3/gogetdoc
        # github.com/alecthomas/gometalinter
        # github.com/josharian/impl
        # golang.org/x/tools/cmd/goimports \

# Various patches for SDK
mtime_file_watcher = https://gist.githubusercontent.com/zeekay/5eba991c39426ca42cbb/raw/8db2e910b89e3927adc9b7c183387186facee17b/mtime_file_watcher.py

bebop     = node_modules/.bin/bebop
coffee	  = node_modules/.bin/coffee
uglifyjs  = node_modules/.bin/uglifyjs
requisite = node_modules/.bin/requisite -g

dev_appserver = $(sdk_path)/dev_appserver.py --skip_sdk_update_check \
											 --datastore_path=~/.gae_datastore.bin \
											 --datastore_consistency_policy=consistent \
											 --dev_appserver_log_level=error

sdk_install_extra = rm -rf $(sdk_path)/demos

# find command differs between bsd/linux thus the two versions
ifeq ($(os), linux)
	packages = $(shell find . -maxdepth 4 -mindepth 2 -name '*.go' \
			   				  -not -path "./.sdk/*" \
			   				  -not -path "./test/*" \
			   				  -not -path "./assets/*" \
			   				  -not -path "./static/*" \
			   				  -not -path "./vendor/*" \
			   				  -not -path "./node_modules/*" \
			   				  -printf '%h\n' | sort -u | sed -e 's/.\//hanzo.io\//')

	test_files = $(shell find ./test -maxdepth 8 -mindepth 2 -name '*.go')
	sed = @sed -i -e
else
	packages = $(shell find . -maxdepth 4 -mindepth 2 -name '*.go' \
			   				  -not -path "./.sdk/*" \
			   				  -not -path "./test/*" \
			   				  -not -path "./assets/*" \
			   				  -not -path "./static/*" \
			   				  -not -path "./vendor/*" \
			   				  -not -path "./node_modules/*" \
			   				  -print0 | xargs -0 -n1 dirname | sort --unique | sed -e 's/.\//hanzo.io\//')
	test_files = $(shell find ./test -type f -maxdepth 8 -mindepth 2 -name '*.go')
	sdk_install_extra := $(sdk_install_extra) && \
						 curl $(mtime_file_watcher) > $(sdk_path)/google/appengine/tools/devappserver2/mtime_file_watcher.py && \
						 pip install macfsevents --upgrade
	sed = @sed -i .bak -e
endif

# set v=1 to enable verbose mode
ifeq ($(v), 1)
	test_verbose = -v=true -- -test.v=true
else
	test_verbose =
endif

# set production=1 to set datastore export/import target to use production
ifeq ($(production), 1)
	project_id = hanzo-production
	gae_config = $(gae_production)
else ifeq ($(sandbox), 1)
	project_id = hanzo-sandbox
	gae_config = $(gae_sandbox)
else
	project_id = hanzo-staging
	gae_config = $(gae_staging)
endif

# force a single module to deploy
ifneq ($(strip $(module)),)
	gae_config = $(module)
endif

datastore_admin_url = https://datastore-admin-dot-$(project_id).appspot.com/_ah/remote_api

test_target = -r=true .sdk/gopath/src/hanzo.io/test
test_focus := $(focus)
ifdef test_focus
	test_target=.sdk/gopath/src/hanzo.io/$(focus)
endif

test_batch := $(batch)
ifdef test_batch
	test_target=$(batch)
endif

export GOROOT := $(goroot)
export GOPATH := $(gopath)

all: deps test install

# BUILD
build: deps
	$(goapp) build $(modules)

# DEPS GO
deps: .sdk/goapp .sdk/go .sdk/gopath/bin/ginkgo .sdk/gopath/bin/glide .sdk/gopath/src/hanzo.io
	$(glide) install

.sdk/goapp:
	wget https://storage.googleapis.com/appengine-sdks/featured/$(sdk).zip
	unzip $(sdk).zip
	rm -rf $(sdk_path)
	mv go_appengine $(sdk_path)
	rm $(sdk).zip
	$(sdk_install_extra)

.sdk/go:
	echo '#!/usr/bin/env bash' > $(sdk_path)/go
	echo '$(sdk_path)/goapp $$@' >> $(sdk_path)/go
	chmod +x $(sdk_path)/go

.sdk/gopath/bin/ginkgo:
	$(goapp) get -u github.com/onsi/ginkgo/ginkgo
	$(goapp) install github.com/onsi/ginkgo/ginkgo

.sdk/gopath/bin/glide:
	$(goapp) get -u github.com/Masterminds/glide
	$(goapp) install github.com/Masterminds/glide

.sdk/gopath/src/hanzo.io:
	mkdir -p $(sdk_path)/gopath/src
	mkdir -p $(sdk_path)/gopath/bin
	mkdir -p $(sdk_path)/vendorpath
	ln -s $(shell pwd) $(sdk_path)/gopath/src/hanzo.io
	ln -s $(shell pwd)/vendor $(sdk_path)/vendorpath/src

# INSTALL
install:
	$(goapp) install $(packages)

install-test:
	echo $(test_files) | xargs $(goapp) test -c

# DEV SERVER
serve:
	$(dev_appserver) $(gae_development)

serve-clear-datastore:
	$(dev_appserver) --clear_datastore=true $(gae_development)

serve-public:
	$(dev_appserver) --host=0.0.0.0 $(gae_development)

serve-no-reload: assets
	$(dev_appserver) $(gae_development)

# GOLANG TOOLS
tools:
	$(goapp) get -u $(tools)
	$(goapp) install $(tools)
	$(gopath)/bin/gocode set autobuild true
	$(gopath)/bin/gocode set propose-builtins true
	$(gopath)/bin/gocode set lib-path "$(gopath_pkg_path):$(gopath_pkg_path)/hanzo.io/vendor:$(goroot_pkg_path)"

# TEST/ BENCH
test:
	@$(ginkgo) $(test_target) -p=true --progress --randomizeAllSpecs --failFast --skipMeasurements --skipPackage=integration $(test_verbose)

test-integration:
	@$(ginkgo) $(test_target) -p=true --progress --randomizeAllSpecs --failFast --skipMeasurements --focus=integration $(test_verbose)

test-watch:
	@$(ginkgo) watch $(test_target) -r=true -p=true -notify --progress --failFast --skipMeasurements $(test_verbose)

bench:
	@$(ginkgo) $(test_target) -p=true --progress --randomizeAllSpecs --failFast --skipPackage=integration $(test_verbose)

test-ci:
	$(ginkgo) $(test_target) -p=true --randomizeAllSpecs --randomizeSuites --failFast --failOnPending --trace

# DEPLOY
deploy: rollback
	# Set env for deploy
	@echo 'package config\n\nvar Env = "$(project_id)"' > config/env.go

	for module in $(gae_config); do \
		$(appcfg.py) update $$module; \
	done
	$(appcfg.py) update_indexes $(firstword $(gae_config))
	$(appcfg.py) update_dispatch $(firstword $(gae_config))

	# Reset env
	@echo 'package config\n\nvar Env = "development"' > config/env.go

rollback:
	for module in $(gae_config); do \
		$(appcfg.py) rollback $$module; \
	done

# EXPORT / Usage: make datastore-export kind=user namespace=bellabeat
datastore-export:
	@mkdir -p _export/
	$(bulkloader.py) --download \
				  	 --bandwidth_limit 1000000000 \
				  	 --rps_limit 10000 \
				  	 --batch_size 250 \
				  	 --http_limit 200 \
				  	 --url $(datastore_admin_url) \
				  	 --config_file util/bulkloader/bulkloader.yaml \
				  	 --db_filename /tmp/bulkloader-$$kind.db \
				  	 --log_file /tmp/bulkloader-$$kind.log \
				  	 --result_db_filename /tmp/bulkloader-result-$$kind.db \
				  	 --namespace $$namespace \
				  	 --kind $$kind \
				  	 --filename _export/$$namespace-$$kind-$(project_id)-$(current_date).csv
	rm -rf /tmp/bulkloader-$$kind.db \
		   /tmp/bulkloader-$$kind.log \
		   /tmp/bulkloader-result-$$kind.db

# IMPORT / Usage: make datastore-import kind=user file=user.csv
datastore-import:
	@$(appcfg.py) upload_data --bandwidth_limit 1000000000 \
						      --rps_limit 10000 \
						      --batch_size 250 \
						      --http_limit 200 \
						      --url $(datastore_admin_url) \
						      --config_file util/bulkloader/bulkloader.yaml \
				  	          --namespace $$namespace \
						      --kind $$kind \
						      --filename $$file \
						      --log_file /tmp/bulkloader-upload-$$kind.log
	rm -rf /tmp/bulkloader-upload-$$kind.log

# Generate config for use with datastore-export target
datastore-config:
	@$(bulkloader.py) --create_config \
				      --url=$(datastore_admin_url) \
				      --namespace $$namespace \
				      --filename=bulkloader.yaml

# Replicate production data to localhost
datastore-replicate:
	$(appcfg.py) download_data --application=s~$(project_id) --url=http://datastore-admin-dot-$(project_id).appspot.com/_ah/remote_api/ --filename=datastore.bin
	$(appcfg.py) --url=http://localhost:8080/_ah/remote_api --filename=datastore.bin upload_data

.PHONY: all bench build compile-js compile-js-min datastore-import \
	datastore-export datastore-config deploy deploy-staging deploy-production \
	deps deps-assets deps-go live-reload serve serve-clear-datastore \
	serve-public test test-integration test-watch tools
