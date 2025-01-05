#!/bin/sh
VENDOR=/system/vendor/unknown321/
BINARY=scrobbler

log()
{
        oldIFS=$IFS
        IFS="
"
        for line in $(echo "${1}"); do
                echo "$(date) ${line}" >> $LOG_FILE
        done
        IFS=$oldIFS
}

uninstall() {
  log "uninstalling ${BINARY}"
  busybox rm -f ${VENDOR}/bin/${BINARY}

  log "uninstalling ${BINARY} service"
  grep -q "init.${BINARY}.rc" "${INITRD_UNPACKED}/init.rc"
  if test $? -eq 0; then
    log "removing service"
    busybox sed -i "/import init.${BINARY}.rc/d" ${INITRD_UNPACKED}/init.rc
  fi
  busybox rm -f ${INITRD_UNPACKED}/init.${BINARY}.rc
}

log "uninstaller for $(cat product_info)"

mount -t ext4 -o rw /emmc@android /system

uninstall

sync
umount /system