#!/bin/sh

#export GOPROXY=https://goproxy.io
#export CGO_ENABLED=0
#export GOOS=linux
#export GOARCH=amd64

export BASEDIR=$(pwd)

echo "============================="
echo "==== building"
echo "============================="

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/mahjong

if [ $? -ne 0 ]
then
    echo "build failed"
    exit -1
fi

echo "============================="
echo "==== packaging"
echo "============================="

tar -cvzf mahjong.tar.gz mahjong ./configs

rm -rf dist
mkdir -p dist
mv ./mahjong.tar.gz dist/
