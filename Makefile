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
	-rm $(OUT) nw-installer/installer/userdata.tar.gz installer/$(OUT) LICENSE_3rdparty
	-rm -rf release

nw-installer/installer/userdata.tar.gz:
	$(MAKE) -C nw-installer prepare
	cp $(OUT) installer/
	$(UPX) -qqq --best installer/$(OUT)
	cat LICENSE LICENSE_3rdparty > nw-installer/installer/windows/LICENSE.txt.user
	echo -n "$(PRODUCT), version " > installer/product_info
	echo $(shell git log -1 --format=%h) >> installer/product_info
	tar -C installer -cf nw-installer/installer/userdata.tar.gz \
		init.scrobbler.rc \
		run.sh \
		product_info \
		scrobbler

uninstaller:
	$(MAKE) -C nw-installer prepare
	cat LICENSE LICENSE_3rdparty > nw-installer/installer/windows/LICENSE.txt.user
	echo -n "$(PRODUCT), version " > uninstaller/product_info
	echo $(shell git log -1 --format=%h) >> uninstaller/product_info
	tar -C uninstaller -cf nw-installer/installer/userdata.tar.gz \
		product_info \
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

release: clean vendor LICENSE_3rdparty $(OUT) nw-installer/installer/userdata.tar.gz
	$(MAKE) -C nw-installer OUTFILE=$(PRODUCT).exe APPNAME=$(PRODUCT) build
	mkdir -p release/installer/
	cd nw-installer/installer/nw-a50/ && tar -czvf nw-a50.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/nw-a40/ && tar -czvf nw-a40.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/nw-a30/ && tar -czvf nw-a30.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/walkmanOne/ && tar -czvf walkmanOne.tar.gz NW_WM_FW.UPG
	mv nw-installer/installer/walkmanOne/walkmanOne.tar.gz release/installer
	mv nw-installer/installer/nw-a30/nw-a30.tar.gz release/installer
	mv nw-installer/installer/nw-a40/nw-a40.tar.gz release/installer
	mv nw-installer/installer/nw-a50/nw-a50.tar.gz release/installer
	mv nw-installer/installer/windows/${PRODUCT}.exe release/installer/${PRODUCT}.$(shell date --iso).$(shell git log -1 --format=%h).exe
	$(MAKE) -C nw-installer OUTFILE=$(PRODUCT).uninstaller.exe APPNAME=$(PRODUCT)-uninstaller clean
	$(MAKE) uninstaller
	$(MAKE) -C nw-installer OUTFILE=$(PRODUCT).uninstaller.exe APPNAME=$(PRODUCT)-uninstaller build
	mkdir -p release/uninstaller
	cd nw-installer/installer/nw-a50/ && tar -czvf nw-a50.uninstaller.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/nw-a40/ && tar -czvf nw-a40.uninstaller.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/nw-a30/ && tar -czvf nw-a30.uninstaller.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/walkmanOne/ && tar -czvf walkmanOne.uninstaller.tar.gz NW_WM_FW.UPG
	mv nw-installer/installer/walkmanOne/walkmanOne.uninstaller.tar.gz release/uninstaller
	mv nw-installer/installer/nw-a50/nw-a50.uninstaller.tar.gz release/uninstaller
	mv nw-installer/installer/nw-a40/nw-a40.uninstaller.tar.gz release/uninstaller
	mv nw-installer/installer/nw-a30/nw-a30.uninstaller.tar.gz release/uninstaller
	mv nw-installer/installer/windows/${PRODUCT}.uninstaller.exe release/uninstaller


push:
	adb wait-for-device push $(OUT) /system/vendor/unknown321/bin/

fast: build push

.DEFAULT_GOAL := build

.PHONY: build uninstaller release
