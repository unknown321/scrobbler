PRODUCT=scrobbler
OUT=$(PRODUCT)
UPX=nw-installer/tools/upx/upx/upx

build: test
	CGO_ENABLED=0 GOARCH=arm GOARM=5 \
	go build -ldflags="-w -s" -trimpath -o $(OUT) .

test:
	go test -v ./...

clean:
	-rm $(OUT) nw-installer/installer/userdata.tar installer/$(OUT)

nw-installer/installer/userdata.tar:
	$(MAKE) -C nw-installer prepare
	cp $(OUT) installer/
	$(UPX) -qqq --best installer/$(OUT)
	tar -C installer -cf nw-installer/installer/userdata.tar \
		init.scrobbler.rc \
		run.sh \
		scrobbler

$(OUT): build

release: $(OUT) nw-installer/installer/userdata.tar
	$(MAKE) -C nw-installer OUTFILE=scrobbler.exe APPNAME=scrobbler

push:
	adb wait-for-device push $(OUT) /system/vendor/unknown321/bin/

fast: build push

.DEFAULT_GOAL := build

.PHONY: build
