#!/bin/bash
# Copyright 2020 Soluble Inc
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

VERSION=$(git describe --tags --dirty --always)

# run with ./build.sh none to skip building executables
# or ./build.sh windows to build only windows, etc
exes="$1"
if [ -z "$exes" ]; then
  # if running as a github action then avoid building exes
  # unless this is a release build
  if [ -n "$GITHUB_EVENT_NAME" -a "$GITHUB_EVENT_NAME" != "release" ]; then
    exes="none"
  fi
fi

echo "Version ${VERSION}"

build_time=$(date -u +%Y-%m-%dT%H:%M:%S+00:00)

ldflags="-ldflags=-X 'github.com/soluble-ai/soluble-cli/pkg/version.Version=${VERSION}' \
-X 'github.com/soluble-ai/soluble-cli/pkg/version.BuildTime=${build_time}'"

set -e

echo "Running go mod tidy -v"
go mod tidy -v

echo "Running go generate"
go generate ./...

# verify integration tests have build tag
if find . -name '*.go' | \
    egrep "integration/.*_test.go" | \
    xargs egrep -c "//go:build integration" | \
    egrep ":0$"; then
    echo "Error: the integration tests listed above should have a '//go:build integration' build constraint"
    exit 1
fi

echo "Running go test (unit tests)"
go test -cover ./...

linter=golangci-lint
if [ -x ./bin/golangci-lint ]; then
    linter=./bin/golangci-lint
fi

if "${linter}" --help > /dev/null 2>&1; then
    echo "Running ${linter}"
    "${linter}" run -E stylecheck -E gosec -E goimports -E misspell -E gocritic \
      -E whitespace -E goprintffuncname \
      -e G402 ; # we turn off TLS verification by option
else
    echo "golangci-lint not available, skipping lint"
fi


echo "Running go test (integration tests)"

go test -tags=integration -timeout 30s ./.../integration

rm -rf dist
mkdir -p dist

IFS=" "

for p in "linux amd64 tar" "windows amd64 zip .exe" "darwin amd64 tar"; do
    if [ -n "$exes" ] && (echo $p | grep -v "$exes" > /dev/null); then
        echo "Skipping build of $p"
        continue
    fi
    read -a os_arch <<< "$p"
    echo "Building $VERSION for ${os_arch[0]} ${os_arch[1]}"
    rm -rf target
    mkdir target
    # need to specify osusergo,netgo tags to actually get a static
    # binary - thanks https://www.arp242.net/static-go.html
    #
    # -trimpath was added to go 1.13 (our minimum build target)
    # which ultimately supports reproducible binary build by
    # removing otherwise hardcoded filesystem paths in the binary.
    set -x
    GOOS=${os_arch[0]} GOARCH=${os_arch[1]} \
        go build -o target/soluble${os_arch[3]} -tags ci,osusergo,netgo -trimpath "$ldflags"
    { set +x; } 2> /dev/null
    cp LICENSE README.md target
    pkg=${os_arch[2]}
    name=soluble_${VERSION}_${os_arch[0]}_${os_arch[1]}
    echo "Packaging $name"
    (
        cd target
        if [ "$pkg" = "tar" ]; then
            tar vcf - * | gzip -9 > ../dist/$name.tar.gz
        elif [ "$pkg" = "zip" ]; then
            zip ../dist/$name.zip *
        fi
    )
done

ls -l dist
