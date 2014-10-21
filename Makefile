sdk = go_appengine_sdk_darwin_amd64-1.9.13

all: deps test

deps: sdk
	gpm install || curl -s https://raw.githubusercontent.com/pote/gpm/v1.3.1/bin/gpm | bash

sdk:
	wget https://storage.googleapis.com/appengine-sdks/featured/$(sdk).zip \
	  && unzip $(sdk).zip \
	  && mv go_appengine sdk \
	  && rm $(sdk).zip

bench:
	goapp test ./... --bench=.

test:
	goapp test ./...

serve:
	goapp serve .

.PHONY: all build deps test serve
