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
	-rm $(OUT) nw-installer/installer/userdata.tar.gz nw-installer/installer/userdata.uninstaller.tar.gz installer/$(OUT) LICENSE_3rdparty
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

nw-installer/installer/userdata.uninstaller.tar.gz:
	$(MAKE) -C nw-installer prepare
	cat LICENSE LICENSE_3rdparty > nw-installer/installer/windows/LICENSE.txt.user
	echo -n "$(PRODUCT), version " > uninstaller/product_info
	echo $(shell git log -1 --format=%h) >> uninstaller/product_info
	tar -C uninstaller -cf nw-installer/installer/userdata.uninstaller.tar.gz \
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

release: clean vendor LICENSE_3rdparty $(OUT) nw-installer/installer/userdata.tar.gz nw-installer/installer/userdata.uninstaller.tar.gz
	# first, build and move uninstaller upgs
	$(MAKE) -C nw-installer OUTFILE=$(PRODUCT).uninstaller.exe APPNAME=$(PRODUCT)-uninstaller USERDATA_FILENAME=userdata.uninstaller.tar.gz build
	mkdir -p release/uninstaller
	cd nw-installer/installer/nw-a50/ && tar -czvf ../../../release/uninstaller/nw-a50.uninstaller.tar.gz NW_WM_FW.UPG && rm NW_WM_FW.UPG
	cd nw-installer/installer/nw-a40/ && tar -czvf ../../../release/uninstaller/nw-a40.uninstaller.tar.gz NW_WM_FW.UPG && rm NW_WM_FW.UPG
	cd nw-installer/installer/nw-a30/ && tar -czvf ../../../release/uninstaller/nw-a30.uninstaller.tar.gz NW_WM_FW.UPG && rm NW_WM_FW.UPG
	cd nw-installer/installer/nw-wm1a/ && tar -czvf ../../../release/uninstaller/nw-wm1a.uninstaller.tar.gz NW_WM_FW.UPG && rm NW_WM_FW.UPG
	cd nw-installer/installer/nw-wm1z/ && tar -czvf ../../../release/uninstaller/nw-wm1z.uninstaller.tar.gz NW_WM_FW.UPG && rm NW_WM_FW.UPG
	cd nw-installer/installer/nw-zx300/ && tar -czvf ../../../release/uninstaller/nw-zx300.uninstaller.tar.gz NW_WM_FW.UPG && rm NW_WM_FW.UPG
	cd nw-installer/installer/dmp-z1/ && tar -czvf ../../../release/uninstaller/dmp-z1.uninstaller.tar.gz NW_WM_FW.UPG && rm NW_WM_FW.UPG
	cd nw-installer/installer/a50z/ && tar -czvf ../../../release/uninstaller/a50z.uninstaller.tar.gz NW_WM_FW.UPG && rm NW_WM_FW.UPG
	cd nw-installer/installer/walkmanOne/ && tar -czvf ../../../release/uninstaller/walkmanOne.uninstaller.tar.gz NW_WM_FW.UPG && rm NW_WM_FW.UPG
	# second, build installer upgs
	$(MAKE) -C nw-installer OUTFILE=$(PRODUCT).exe APPNAME=$(PRODUCT) build
	# next, build installer (with uninstaller included)
	$(MAKE) -C nw-installer OUTFILE=$(PRODUCT).exe APPNAME=$(PRODUCT) win
	# finally, move installer upg and exe files
	mkdir -p release/installer/
	cd nw-installer/installer/nw-a50/ && tar -czvf ../../../release/installer/nw-a50.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/nw-a40/ && tar -czvf ../../../release/installer/nw-a40.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/nw-a30/ && tar -czvf ../../../release/installer/nw-a30.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/nw-wm1a/ && tar -czvf ../../../release/installer/nw-wm1a.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/nw-wm1z/ && tar -czvf ../../../release/installer/nw-wm1z.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/nw-zx300/ && tar -czvf ../../../release/installer/nw-zx300.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/dmp-z1/ && tar -czvf ../../../release/installer/dmp-z1.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/a50z/ && tar -czvf ../../../release/installer/a50z.tar.gz NW_WM_FW.UPG
	cd nw-installer/installer/walkmanOne/ && tar -czvf ../../../release/installer/walkmanOne.tar.gz NW_WM_FW.UPG
	mv nw-installer/installer/windows/$(PRODUCT).exe release/installer/$(PRODUCT).$(shell date --iso).$(shell git log -1 --format=%h).exe

push:
	adb wait-for-device push $(OUT) /system/vendor/unknown321/bin/

fast: build push

.DEFAULT_GOAL := build

.PHONY: build uninstaller release
