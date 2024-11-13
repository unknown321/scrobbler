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

install() {
  log "installing ${BINARY}"
  mkdir -p ${VENDOR}/bin/
  cp ${BINARY} ${VENDOR}/bin/
  chmod 0744 ${VENDOR}/bin/${BINARY}

  log "installing ${BINARY} service"
  cp "init.${BINARY}.rc" ${INITRD_UNPACKED}/
  chmod 0600 "${INITRD_UNPACKED}/init.${BINARY}.rc"
  grep -q "init.${BINARY}.rc" "${INITRD_UNPACKED}/init.rc"
  if test $? -ne 0; then
    log "adding service"
    echo -e "import init.${BINARY}.rc\n$(cat ${INITRD_UNPACKED}/init.rc)" > "${INITRD_UNPACKED}/init.rc"
  fi
}

mount -t ext4 -o rw /emmc@android /system

install

sync
umount /system