#!/usr/bin/env bash
#
# Tag, push and release to github
#
# Prerequisites:
# - commited code
# - $GITHUB_TOKEN


function usage {
    echo "Release does a git tag, push and creates a github release."
    echo "Usage: GITHUB_TOKEN=xxx $0 <version> <title>"
    echo "  <version>:  the version to tag and release."
    echo "  <title>:    the title of the release."
    exit $1
} 


if [[ -z $GITHUB_TOKEN ]]; then
    echo "Error: \$GITHUB_TOKEN needs to be set"
    usage 1
fi

VERSION=$1
TITLE=$2

if [[ -z "$VERSION" || -z "$TITLE" ]]; then
    echo "Error: you need to specify both VERSION and TITLE"
    usage 1
fi

echo "Tag and push code"
git tag $VERSION
git push origin master --tag

echo "Add binaries to github and release"
gothub release -u mmlt -r apigw -t $VERSION --name $TITLE --pre-release
gothub upload -u mmlt -r apigw -t $VERSION --name "apigw-linux-amd64" --file ./apigw
