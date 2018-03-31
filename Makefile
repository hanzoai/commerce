os				= $(shell uname | tr '[A-Z]' '[a-z]')
pwd				= $(shell pwd)
platform		= $(os)_amd64
sdk				= go_appengine_sdk_$(platform)-1.9.64
sdk_path		= $(pwd)/sdk
goroot			= $(sdk_path)/goroot-1.9
gopath			= $(sdk_path)/gopath
goroot_pkg_path = $(goroot)/pkg/$(platform)_appengine/
gopath_pkg_path = $(gopath)/pkg/$(platform)_appengine/
project_path 	= $(gopath)/src/hanzo.io
current_date	= $(shell date +"%Y-%m-%d")

appcfg.py 		= python2 $(sdk_path)/appcfg.py --skip_sdk_update_check
bulkloader.py   = python2 $(sdk_path)/bulkloader.py
goapp			= $(goroot)/bin/goapp

gover 			= $(gopath)/bin/gover
goveralls       = $(gopath)/bin/goveralls
govendor		= GOROOT=$(goroot) GOPATH=$(gopath) PATH=$(sdk_path):$$PATH cd $(project_path); $(gopath)/bin/govendor
ginkgo			= GOROOT=$(goroot) GOPATH=$(gopath) PATH=$(sdk_path):$$PATH $(gopath)/bin/ginkgo

modules	= hanzo.io/config \
	      hanzo.io/api

gae_development = config/development \
				  api/app.dev.yaml

gae_staging = config/staging \
			  api/app.staging.yaml

gae_production = config/production \
				 api

gae_sandbox = config/sandbox \
			  api/app.sandbox.yaml

tools = github.com/nsf/gocode \
        github.com/alecthomas/gometalinter \
        github.com/fatih/motion \
        github.com/golang/lint/golint \
        github.com/josharian/impl \
        github.com/jstemmer/gotags \
        github.com/kisielk/errcheck \
        github.com/klauspost/asmfmt/cmd/asmfmt \
        github.com/rogpeppe/godef \
        github.com/zmb3/gogetdoc \
        golang.org/x/tools/cmd/goimports \
        golang.org/x/tools/cmd/gorename \
        golang.org/x/tools/cmd/guru

# Various patches for SDK
mtime_file_watcher = https://gist.githubusercontent.com/zeekay/5eba991c39426ca42cbb/raw/8db2e910b89e3927adc9b7c183387186facee17b/mtime_file_watcher.py

dev_appserver = python2 $(sdk_path)/dev_appserver.py --skip_sdk_update_check \
											 --dev_appserver_log_level=debug \
											 --datastore_path=$(sdk_path)/.datastore.bin \
    										 --enable_task_running=true \
											 --admin_port=8000 \
											 --port=8080

sdk_install_extra = rm -rf sdk/goroot-1.6 \
					rm -rf sdk/goroot-1.8 \
					rm -rf sdk/php \
					rm -rf sdk/demos

# find command differs between bsd/linux thus the two versions
ifeq ($(os), linux)
	packages = $(shell find . -maxdepth 4 -mindepth 2 -name '*.go' \
			   				  -not -path "./sdk/*" \
			   				  -not -path "./test/*" \
			   				  -not -path "./vendor/*" \
			   				  -printf '%h\n' | sort -u | sed -e 's/.\//hanzo.io\//')
	sed = @sed -i -e
else
	packages = $(shell find . -maxdepth 4 -mindepth 2 -name '*.go' \
			   				  -not -path "./sdk/*" \
			   				  -not -path "./test/*" \
			   				  -not -path "./vendor/*" \
			   				  -print0 | xargs -0 -n1 dirname | sort --unique | sed -e 's/.\//hanzo.io\//')
	sed = @sed -i .bak -e
	sdk_install_extra := $(sdk_install_extra) \
						 curl $(mtime_file_watcher) > $(sdk_path)/google/appengine/tools/devappserver2/mtime_file_watcher.py && \
						 pip2 install macfsevents --upgrade
endif

# set v=1 to enable verbose mode
ifeq ($(v), 1)
	test_verbose = --v --progress -- -test.v=true
else
	test_verbose =
endif

project_env = development
project_id  = None

# set production=1 to set datastore export/import target to use production
ifeq ($(production), 1)
	project_env = production
	project_id  = crowdstart-us
	gae_config  = $(gae_production)
else ifeq ($(sandbox), 1)
	project_env = sandbox
	project_id  = crowdstart-sandbox
	gae_config  = $(gae_sandbox)
else
	project_env = staging
	project_id  = crowdstart-staging
	gae_config  = $(gae_staging)
endif

# force a single module to deploy
ifneq ($(strip $(module)),)
	gae_config = $(module)
endif

datastore_admin_url = https://datastore-admin-dot-$(project_id).appspot.com/_ah/remote_api

test_target = -r=true test
test_focus := $(focus)
ifdef test_focus
	test_target=$(focus)
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

# CLEAN
clean:
	rm -rf sdk
	rm -rf vendor/github.com
	rm -rf vendor/golang.org
	rm -rf vendor/google.golang.org
	rm -rf vendor/gopkg.in

# DEPS
deps: sdk deps-tools deps-go

deps-tools: sdk/gopath/bin/ginkgo sdk/gopath/bin/govendor

deps-go: update-env
	$(govendor) sync

sdk: sdk/go sdk/gopath/src/hanzo.io
	wget https://storage.googleapis.com/appengine-sdks/featured/$(sdk).zip
	unzip -q $(sdk).zip
	mv go_appengine $(sdk_path)
	rm $(sdk).zip
	$(sed) 's/15/120/g' sdk/goroot-1.9/src/appengine/aetest/instance.go
	$(install_sdk_extra)

sdk/go:
	ln -s goroot-1.9/bin/goapp $(sdk_path)/go

sdk/gopath/src/hanzo.io:
	mkdir -p $(sdk_path)/gopath/src
	mkdir -p $(sdk_path)/gopath/bin
	ln -s ../../../ $(sdk_path)/gopath/src/hanzo.io

sdk/gopath/bin/ginkgo:
	$(goapp) get -u github.com/onsi/ginkgo
	$(goapp) install github.com/onsi/ginkgo/ginkgo

sdk/gopath/bin/govendor:
	$(goapp) get -u github.com/kardianos/govendor
	$(goapp) install github.com/kardianos/govendor

# INSTALL
install:
	$(goapp) install $(packages)

# DEV SERVER
serve: assets update-env
	$(bebop) &
	$(dev_appserver) $(gae_development)

serve-clear-datastore: assets update-env
	$(bebop) &
	$(dev_appserver) --clear_datastore=true $(gae_development)

serve-public: assets update-env
	$(bebop) &
	$(dev_appserver) --host=0.0.0.0 $(gae_development)

serve-no-reload: assets update-env
	$(dev_appserver) $(gae_development)

# GOLANG TOOLS
tools:
	@echo If you have issues building:
	@echo "  rm sdk/gopath/src/golang.org/x/tools/imports/fastwalk_unix.go"
	@echo "  rm sdk/gopath/src/github.com/alecthomas/gometalinter/vendor/gopkg.in/alecthomas/kingpin.v3-unstable/guesswidth_unix.go"
	@echo
	$(goapp) get $(tools)
	$(goapp) install $(tools)
	$(gopath)/bin/gocode set propose-builtins true
	$(gopath)/bin/gocode set lib-path "$(gopath_pkg_path):$(goroot_pkg_path)"

# TEST/ BENCH
test: update-env-test
	$(ginkgo) $(test_target) --compilers=2 --randomizeAllSpecs --failFast --trace --skipMeasurements --skipPackage=integration $(test_verbose)

test-watch: update-env-test
	$(ginkgo) watch -r=true --compilers=2 --failFast --trace $(test_verbose)

bench: update-env-test
	$(ginkgo) $(test_target) --compilers=2 --randomizeAllSpecs --failFast --trace --skipPackage=integration $(test_verbose)

test-ci: update-env-test
	cd $(project_path); $(ginkgo) $(test_target) --randomizeAllSpecs --randomizeSuites --failFast --failOnPending --trace $(test_verbose)

coverage:
	# $(gover) test/ coverage.out
	# $(goveralls) -coverprofile=coverage.out -service=circle-ci -repotoken=$(COVERALLS_REPO_TOKEN)

# DEPLOY
auth:
	@echo If you have issues authenticating try:
	@echo "   gcloud components reinstall"
	@echo "	 rm ~/.appcfg*"
	gcloud auth login
	$(appcfg.py) list_versions config/staging

deploy: update-env rollback
	for module in $(gae_config); do \
		$(appcfg.py) update $$module; \
	done
	$(appcfg.py) update_indexes $(firstword $(gae_config))

update-dispatch:
	$(appcfg.py) update_dispatch config/$(project_env)

update-env:
	@printf 'package config\n\nvar Env = "$(project_env)"' > config/env.go

update-env-test:
	@printf 'package config\n\nvar Env = "test"' > config/env.go

rollback:
	for module in $(gae_config); do \
		$(appcfg.py) rollback $$module; \
	done

# EXPORT / Usage: make datastore-export kind=user namespace=bellabeat
datastore-export:
	@mkdir -p _export/
	$(appcfg.py) download_data \
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
	$(appcfg.py) create_bulkloader_config \
				 --url=$(datastore_admin_url) \
				 --filename=bulkloader.yaml

# Replicate production data to localhost
datastore-replicate:
	$(appcfg.py) download_data --application=s~$(project_id) --url=http://datastore-admin-dot-$(project_id).appspot.com/_ah/remote_api/ --filename=datastore.bin
	$(appcfg.py) --url=http://localhost:8080/_ah/remote_api --filename=datastore.bin upload_data

# Helpers to store and retrieve build artifacts
artifact-download:
	buildkite-agent artifact download sdk.tar . && tar -xf sdk.tar || echo no sdk artifact found
	buildkite-agent artifact download vendor.tar . && tar -xf vendor.tar || echo no vendor artifact found

artifact-download-prev : build_id = $(shell curl -H "Authorization: Bearer 08a7fd928cc9062dd7522f92f9781fb0d7ea822f" https://api.buildkite.com/v2/organizations/hanzo/pipelines/platform/builds/$$(( $$BUILDKITE_BUILD_NUMBER - 1 )) | jq -r .id)
artifact-download-prev:
	buildkite-agent artifact download sdk.tar . --build $(build_id) && tar -xf sdk.tar || echo no sdk artifact found
	buildkite-agent artifact download vendor.tar . --build $(build_id) && tar -xf vendor.tar || echo no vendor artifact found

artifact-upload:
	tar -cf sdk.tar sdk
	tar -cf vendor.tar vendor
	buildkite-agent artifact upload '*.tar'

.PHONY: all auth bench build buildkite-artifact-download \
	buildkite-artifact-upload compile-js compile-js-min compile-css \
	compile-css-min datastore-import datastore-export datastore-config \
	deploy \ deploy-staging deploy-production deps deps-assets deps-go \
	live-reload serve serve-clear-datastore serve-public test \
	test-integration test-watch tools
