#!/usr/bin/env bash

PATH=$PATH:$HOME/go/bin

pushd ../pflow-editor/
npm run build
popd
rm -rf ./public/p
rm -rf ./public
mkdir ./public
mv ../pflow-editor/build ./public/p
# NOTE: must have rice tool installed
if [[ -x rice ]] ; then
    echo "found rice: $(which rice)" 
else
    echo "installing rice"
    go install github.com/GeertJohan/go.rice/rice@latest
fi;
rice embed-go
go build #-ldflags "-s"
echo "remember to update the script tag main.<build>.js ./app/app.go"
