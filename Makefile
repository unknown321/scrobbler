PRODUCT=scrobbler
OUT=$(PRODUCT)
UPX=nw-installer/tools/upx/upx/upx
ECHO=/usr/bin/echo

build: test
	CGO_ENABLED=0 GOARCH=arm GOARM=5 \
	go build -ldflags="-w -s" -trimpath -o $(OUT) .

test:
	go test -v ./...

clean:
	$(MAKE) -C nw-installer OUTFILE=$(PRODUCT).exe APPNAME=$(PRODUCT) clean
	-rm $(OUT) nw-installer/installer/userdata.tar installer/$(OUT) LICENSE_3rdparty

nw-installer/installer/userdata.tar:
	$(MAKE) -C nw-installer prepare
	cp $(OUT) installer/
	$(UPX) -qqq --best installer/$(OUT)
	cat LICENSE LICENSE_3rdparty > nw-installer/installer/windows/LICENSE.txt.user
	tar -C installer -cf nw-installer/installer/userdata.tar \
		init.scrobbler.rc \
		run.sh \
		scrobbler

LICENSE_3rdparty:
	@$(ECHO) -e "\n***\nsqlite:\n" > $@
	@cat vendor/modernc.org/sqlite/LICENSE >> $@
	@$(ECHO) -e "\n***\ngolang text module\n" >> $@
	@cat vendor/golang.org/x/text/LICENSE >> $@
	@$(ECHO) -e "\n***\ngoogle go-cmp module\n" >> $@
	@cat vendor/github.com/google/go-cmp/LICENSE >> $@

$(OUT): build

release: clean LICENSE_3rdparty $(OUT) nw-installer/installer/userdata.tar
	$(MAKE) -C nw-installer OUTFILE=$(PRODUCT).exe APPNAME=$(PRODUCT) build

push:
	adb wait-for-device push $(OUT) /system/vendor/unknown321/bin/

fast: build push

.DEFAULT_GOAL := build

.PHONY: build
