#!/bin/sh

echo starting servers.
ssh pitter24 "$HOME/mygo/src/github.com/relab/smartMerge/server/servers.sh" &
ssh pitter25 "$HOME/mygo/src/github.com/relab/smartMerge/server/servers.sh" &
ssh pitter26 "$HOME/mygo/src/github.com/relab/smartMerge/server/servers.sh" &

export SM=$HOME/mygo/src/github.com/relab/smartMerge

sleep 1

cd $SM

echo starting Writers
(client/client -conf client/addrList -alg=sm -mode=bench -contW -size=4000 -nclients=5 -id=5 -initsize=12 -gc-off > logfile &)

echo starting Reconfigurers
client/client -conf client/addrList -alg=sm -mode=exp -rm -nclients=2 -initsize=12 -gc-off -elog

sleep 1
echo stopping Writers
killall $SM/client/client 
 
ssh pitter24 "pkill -u ljehl"
ssh pitter25 "pkill -u ljehl"
ssh pitter26 "pkill -u ljehl"

