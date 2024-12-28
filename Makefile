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
	-rm -rf release

nw-installer/installer/userdata.tar:
	$(MAKE) -C nw-installer prepare
	cp $(OUT) installer/
	$(UPX) -qqq --best installer/$(OUT)
	cat LICENSE LICENSE_3rdparty > nw-installer/installer/windows/LICENSE.txt.user
	tar -C installer -cf nw-installer/installer/userdata.tar \
		init.scrobbler.rc \
		run.sh \
		scrobbler

uninstaller:
	$(MAKE) -C nw-installer prepare
	cat LICENSE LICENSE_3rdparty > nw-installer/installer/windows/LICENSE.txt.user
	tar -C uninstaller -cf nw-installer/installer/userdata.tar \
		run.sh

LICENSE_3rdparty:
	@$(ECHO) -e "\n***\nsqlite:\n" > $@
	@cat vendor/modernc.org/sqlite/LICENSE >> $@
	@$(ECHO) -e "\n***\ngolang text module\n" >> $@
	@cat vendor/golang.org/x/text/LICENSE >> $@
	@$(ECHO) -e "\n***\ngoogle go-cmp module\n" >> $@
	@cat vendor/github.com/google/go-cmp/LICENSE >> $@

$(OUT): build

vendor:
	go mod vendor

release: clean vendor LICENSE_3rdparty $(OUT) nw-installer/installer/userdata.tar
	$(MAKE) -C nw-installer OUTFILE=$(PRODUCT).exe APPNAME=$(PRODUCT) build
	mkdir -p release/installer
	cd nw-installer/installer/stock/ && tar -czvf stock.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/walkmanOne/ && tar -czvf walkmanOne.tar.gz NW_WM_FW.UPG
	mv nw-installer/installer/walkmanOne/walkmanOne.tar.gz release/installer
	mv nw-installer/installer/stock/stock.tar.gz release/installer
	mv nw-installer/installer/windows/${PRODUCT}.exe release/installer
	$(MAKE) -C nw-installer OUTFILE=$(PRODUCT).uninstaller.exe APPNAME=$(PRODUCT)-uninstaller clean
	$(MAKE) uninstaller
	$(MAKE) -C nw-installer OUTFILE=$(PRODUCT).uninstaller.exe APPNAME=$(PRODUCT)-uninstaller build
	mkdir -p release/uninstaller
	cd nw-installer/installer/stock/ && tar -czvf stock.uninstaller.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/walkmanOne/ && tar -czvf walkmanOne.uninstaller.tar.gz NW_WM_FW.UPG
	mv nw-installer/installer/walkmanOne/walkmanOne.uninstaller.tar.gz release/uninstaller
	mv nw-installer/installer/stock/stock.uninstaller.tar.gz release/uninstaller
	mv nw-installer/installer/windows/${PRODUCT}.uninstaller.exe release/uninstaller


push:
	adb wait-for-device push $(OUT) /system/vendor/unknown321/bin/

fast: build push

.DEFAULT_GOAL := build

.PHONY: build uninstaller release
