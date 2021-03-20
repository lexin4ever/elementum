Building Elementum for release:

```
#!/bin/bash

set -e

TAG=$(git describe --tags)

export GH_TOKEN=aaaaaaaaaaaaaaaaaaaaaaaaaaa # This is an access token from Github
export PATH=$HOME/go/bin:/usr/lib/go-1.13/bin/:$PATH
export GOPATH=$HOME/go
unset GOROOT

git checkout master

rm -rf ~/go/src/github.com/elgatito/elementum
ln -s ~/workspace/elementum ~/go/src/github.com/elgatito/elementum
cd ~/go/src/github.com/elgatito/elementum

sudo -S true

make all

if [[ $TAG != *-* ]]
then
    # Push binaries to github
	./push-binaries.sh
fi
```