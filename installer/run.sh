#!/bin/sh
VENDOR=/system/vendor/unknown321/
BINARY=scrobbler

GREP="/xbin/busybox grep"
MKDIR="/xbin/busybox mkdir"
CP="/xbin/busybox cp"
RM="/xbin/busybox rm"
CHMOD="/xbin/busybox chmod"

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

install() {
  log "installing ${BINARY}"
  ${MKDIR} -p ${VENDOR}/bin/
  ${CP} ${BINARY} ${VENDOR}/bin/
  ${CHMOD} 0755 ${VENDOR}/bin/${BINARY}

  log "installing ${BINARY} service"
  ${CP} "init.${BINARY}.rc" ${INITRD_UNPACKED}/
  ${CHMOD} 0600 "${INITRD_UNPACKED}/init.${BINARY}.rc"
  ${GREP} -q "init.${BINARY}.rc" "${INITRD_UNPACKED}/init.rc"
  if test $? -ne 0; then
    log "adding service"
    echo -e "import init.${BINARY}.rc\n$(cat ${INITRD_UNPACKED}/init.rc)" > "${INITRD_UNPACKED}/init.rc"
  fi
}

log "installer for $(cat product_info)"

mount -t ext4 -o rw /emmc@android /system

install

sync
umount /system
