#!/usr/bin/env sh

trap 'exit 0' INT TERM
MY_TIME=0
END=$((MY_TIME+TIMEOUT))
while [ $MY_TIME -lt $END ]; do
  echo "Wait $((END-MY_TIME))s and exit"
  MY_TIME=$((MY_TIME+1))
  sleep 1
done
exit 0