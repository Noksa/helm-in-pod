#!/usr/bin/env sh

trapMe() {
  set +e
  TRAP_TIME=0
  TRAP_END_TIME=$((TRAP_TIME+180))
  while [ $TRAP_TIME -lt $TRAP_END_TIME ]; do
    echo "Sending INT and TERM to all processes except PID 1"
    kill -s INT -1 2>/dev/null
    kill -s TERM -1 2>/dev/null
    RES="$(ps aux | grep "${HOME}/wrapped-script.sh" | grep -v "grep" | xargs)"
    if [ -z "${RES}" ]; then
      RES="$(ps aux | grep "helm" | grep -v "grep" | xargs)"
    fi
    if [ -z "${RES}" ]; then
      RES="$(ps aux | grep "kubectl" | grep -v "grep" | xargs)"
    fi
    if [ -z "${RES}" ]; then
      echo "exiting - no wrapped-script/helm/kubectl processes found"
      exit 1
    fi
    TRAP_TIME=$((TRAP_TIME+3))
    sleep 3
  done
  exit 1
}


trap 'trapMe' INT TERM
set -eu
MY_TIME=0
END=$((MY_TIME+TIMEOUT))
touch /tmp/ready
SCRIPT_PATH="${HOME}/wrapped-script.sh"
while [ $MY_TIME -lt $END ]; do
  #echo "Waiting ${SCRIPT_PATH}"
  if [ ! -f "${SCRIPT_PATH}" ]; then
    sleep 1
    continue
  fi
  break
done

#echo "#### EXECUTION STARTED ####"
sh -eu "${SCRIPT_PATH}"
exit $?