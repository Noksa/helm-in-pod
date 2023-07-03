#!/usr/bin/env sh

trapMe() {
  set +e
  TRAP_TIME=0
  TRAP_END_TIME=$((TRAP_TIME+180))
  while [ $TRAP_TIME -lt $TRAP_END_TIME ]; do
    RES=$(ps aux | grep helm | grep -v "grep" | wc -l)
    if [ "${RES}" = "0" ]; then
      echo "No helm processes found, exiting"
      exit 0
    fi
    PIDS=$(pidof helm)
    for PID in $PIDS; do
      echo "Sending TERM to helm process with ${PID} pid"
      kill -term ${PID}
    done
    echo "Waiting for $((TRAP_END_TIME-TRAP_TIME))s until all helm processes die"
    TRAP_TIME=$((TRAP_TIME+3))
    sleep 3
  done
  exit 0
}


trap 'trapMe' INT TERM
MY_TIME=0
END=$((MY_TIME+TIMEOUT))
while [ $MY_TIME -lt $END ]; do
  echo "Wait $((END-MY_TIME))s and exit"
  MY_TIME=$((MY_TIME+1))
  sleep 1
done
exit 0