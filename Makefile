os				= $(shell uname | tr '[A-Z]' '[a-z]')
pwd				= $(shell pwd)
platform		= $(os)_amd64
sdk				= go_appengine_sdk_$(platform)-1.9.62
sdk_path		= $(pwd)/.sdk
goroot			= $(sdk_path)/goroot
gopath			= $(sdk_path)/gopath
goroot_pkg_path = $(goroot)/pkg/$(platform)_appengine/
gopath_pkg_path = $(gopath)/pkg/$(platform)_appengine/
current_date	= $(shell date +"%Y-%m-%d")

appcfg.py 		= python2 $(sdk_path)/appcfg.py --skip_sdk_update_check
bulkloader.py   = python2 $(sdk_path)/bulkloader.py
goapp			= $(sdk_path)/goapp
gover 			= $(gopath)/bin/gover
goveralls       = $(gopath)/bin/goveralls

ginkgo			= GOPATH=$(gopath) PATH=$(sdk_path):$$PATH $(gopath)/bin/ginkgo
gpm				= GOPATH=$(gopath) PATH=$(sdk_path):$$PATH $(sdk_path)/gpm

deps	= $(shell cat Godeps | cut -d ' ' -f 1)
modules	= hanzo.io/analytics \
		  hanzo.io/api \
		  hanzo.io/dash

gae_development = config/development \
				  analytics/app.dev.yaml \
				  api/app.dev.yaml \
				  dash/app.dev.yaml

gae_staging = config/staging \
			  analytics/app.staging.yaml \
			  api/app.staging.yaml \
			  dash/app.staging.yaml

gae_production = config/production \
				 analytics \
				 api \
				 dash

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

bebop    = node_modules/.bin/bebop
coffee	 = node_modules/.bin/coffee
uglifyjs = node_modules/.bin/uglifyjs

requisite	   = node_modules/.bin/requisite -g
requisite_opts = assets/js/api/api.coffee \
				 assets/js/dash/dash.coffee \
				 -o static/js/api.js \
				 -o static/js/dash.js

# requisite_opts_min = --strip-debug --minifier uglify
requisite_opts_min = --strip-debug

stylus		= node_modules/.bin/stylus
stylus_opts = assets/css/theme/theme.styl \
			  assets/css/dash/dash.styl \
			  -o static/css
stylus_opts_min = -u csso-stylus -c

autoprefixer = node_modules/.bin/autoprefixer-cli
autoprefixer_opts = -b 'ie > 8, firefox > 24, chrome > 30, safari > 6, opera > 17, ios > 6, android > 4' \
					static/css/theme.css \
					static/css/dash.css

dev_appserver = python2 $(sdk_path)/dev_appserver.py --skip_sdk_update_check \
											 --dev_appserver_log_level=error
											 --datastore_path=$(sdk_path)/.datastore.bin \

sdk_install_extra = rm -rf $(sdk_path)/demos

# find command differs between bsd/linux thus the two versions
ifeq ($(os), linux)
	packages = $(shell find . -maxdepth 4 -mindepth 2 -name '*.go' \
			   				  -not -path "./.sdk/*" \
			   				  -not -path "./test/*" \
			   				  -not -path "./assets/*" \
			   				  -not -path "./static/*" \
			   				  -not -path "./node_modules/*" \
			   				  -printf '%h\n' | sort -u | sed -e 's/.\//hanzo.io\//')
	sed = @sed -i -e
else
	packages = $(shell find . -maxdepth 4 -mindepth 2 -name '*.go' \
			   				  -not -path "./.sdk/*" \
			   				  -not -path "./test/*" \
			   				  -not -path "./assets/*" \
			   				  -not -path "./static/*" \
			   				  -not -path "./node_modules/*" \
			   				  -print0 | xargs -0 -n1 dirname | sort --unique | sed -e 's/.\//hanzo.io\//')
	# sdk_install_extra := $(sdk_install_extra) && \
	# 					 curl $(mtime_file_watcher) > $(sdk_path)/google/appengine/tools/devappserver2/mtime_file_watcher.py && \
	# 					 pip2 install macfsevents --upgrade
	sed = @sed -i .bak -e
endif

# set v=1 to enable verbose mode
ifeq ($(v), 1)
	test_verbose = -v -- -test.v
else
	test_verbose =
endif

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

test_target = -r=true
test_focus := $(focus)
ifdef test_focus
	test_target=test/$(focus)
endif

test_batch := $(batch)
ifdef test_batch
	test_target=$(batch)
endif

export GOROOT := $(goroot)
export GOPATH := $(gopath)

all: deps test install

# ASSETS
assets: deps-assets compile-css compile-js

assets-min: deps-assets compile-css-min compile-js-min

compile-js:
	$(requisite) $(requisite_opts)
	$(coffee) -bc -o static/js assets/js/api/mailinglist.coffee
	$(requisite) node_modules/crowdstart-analytics/lib/index.js -o static/js/analytics/analytics.js
	cp node_modules/crowdstart-analytics/lib/snippet.js static/js/analytics
	cp node_modules/crowdstart-analytics/lib/bundle.js static/js/analytics

compile-js-min: compile-js
	$(uglifyjs) static/js/api.js -o static/js/api.min.js -c
	$(uglifyjs) static/js/analytics/analytics.js -o static/js/analytics/analytics.min.js -c -m
	$(uglifyjs) static/js/analytics/bundle.js -o static/js/analytics/bundle.min.js -c -m
	$(uglifyjs) static/js/analytics/snippet.js -o static/js/analytics/snippet.min.js -c -m
	$(uglifyjs) static/js/dash.js -o static/js/dash.min.js -c
	@mv static/js/api.min.js static/js/api.js
	@mv static/js/analytics/analytics.min.js static/js/analytics/analytics.js
	@mv static/js/analytics/bundle.min.js static/js/analytics/bundle.js
	@mv static/js/analytics/snippet.min.js static/js/analytics/snippet.js
	@mv static/js/dash.min.js static/js/dash.js

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
	npm update

# DEPS GO
deps-go: .sdk .sdk/go .sdk/gpm .sdk/gopath/bin/ginkgo .sdk/gopath/src/hanzo.io update-env
	$(gpm) install

.sdk:
	wget https://storage.googleapis.com/appengine-sdks/featured/$(sdk).zip
	unzip $(sdk).zip
	mv go_appengine $(sdk_path)
	rm $(sdk).zip
	$(sdk_install_extra)

.sdk/go:
	echo '#!/usr/bin/env bash' > $(sdk_path)/go
	echo '$(sdk_path)/goapp $$@' >> $(sdk_path)/go
	chmod +x $(sdk_path)/go

.sdk/gpm:
	curl -s https://raw.githubusercontent.com/pote/gpm/v1.4.0/bin/gpm > .sdk/gpm
	chmod +x .sdk/gpm

.sdk/gopath/bin/ginkgo:
	$(gpm) install
	$(goapp) install github.com/onsi/ginkgo/ginkgo

.sdk/gopath/src/hanzo.io:
	mkdir -p $(sdk_path)/gopath/src
	mkdir -p $(sdk_path)/gopath/bin
	ln -s $(shell pwd) $(sdk_path)/gopath/src/hanzo.io

# INSTALL
install: install-deps
	$(goapp) install $(modules) $(packages)

install-deps:
	$(goapp) install $(deps)

# DEV SERVER
update-env:
	@echo 'package config\n\nvar Env = "development"' > config/env.go

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
	@echo "  rm .sdk/gopath/src/golang.org/x/tools/imports/fastwalk_unix.go"
	@echo "  rm .sdk/gopath/src/github.com/alecthomas/gometalinter/vendor/gopkg.in/alecthomas/kingpin.v3-unstable/guesswidth_unix.go"
	@echo
	$(goapp) get $(tools)
	$(goapp) install $(tools)
	$(gopath)/bin/gocode set propose-builtins true
	$(gopath)/bin/gocode set lib-path "$(gopath_pkg_path):$(goroot_pkg_path)"

# TEST/ BENCH
test: install
	@$(ginkgo) $(test_target) -p=true -progress --randomizeAllSpecs --failFast --trace --skipMeasurements --skipPackage=integration $(test_verbose)

test-watch:
	@$(ginkgo) watch -r=true -p=true -progress --failFast --trace $(test_verbose)

bench: install
	@$(ginkgo) $(test_target) -p=true -progress --randomizeAllSpecs --failFast --trace --skipPackage=integration $(test_verbose)

test-ci:
	$(ginkgo) $(test_target) -p=true --randomizeAllSpecs --randomizeSuites --failFast --failOnPending --trace

coverage:
	# $(gover) test/ coverage.out
	# $(goveralls) -coverprofile=coverage.out -service=circle-ci -repotoken=$(COVERALLS_REPO_TOKEN)

# DEPLOY

# To re-auth you might need to:
# 	gcloud components reinstall
# 	rm ~/.appcfg*

auth:
	gcloud auth login
	$(appcfg.py) list_versions config/staging

deploy: assets-min deploy-app

deploy-debug: assets deploy-app

deploy-app: rollback
	# Set env for deploy
	@echo 'package config\n\nvar Env = "$(project_id)"' > config/env.go

	for module in $(gae_config); do \
		$(appcfg.py) update $$module; \
	done
	$(appcfg.py) update_indexes $(firstword $(gae_config))

deploy-default: rollback
	# Set env for deploy
	@echo 'package config\n\nvar Env = "$(project_id)"' > config/env.go

	$(appcfg.py) update config/production
	$(appcfg.py) update_indexes $(firstword $(gae_config))

deploy-dash: assets-min rollback
	# Set env for deploy
	@echo 'package config\n\nvar Env = "$(project_id)"' > config/env.go

	$(appcfg.py) update dash
	$(appcfg.py) update_indexes $(firstword $(gae_config))

deploy-api: assets-min rollback
	# Set env for deploy
	@echo 'package config\n\nvar Env = "$(project_id)"' > config/env.go

	$(appcfg.py) update api
	$(appcfg.py) update_indexes $(firstword $(gae_config))

update-dispatch:
	$(appcfg.py) update_dispatch config/$(project_env)

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

.PHONY: all auth bench build compile-js compile-js-min compile-css compile-css-min \
	datastore-import datastore-export datastore-config deploy deploy-staging \
	deploy-production deps deps-assets deps-go live-reload serve serve-clear-datastore \
	serve-public test test-integration test-watch tools
