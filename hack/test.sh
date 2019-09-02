#!/bin/bash
set -e

covermode=count
output=test.cov
echo "mode: ${covermode}" > ${output}

for dir in $(find . -maxdepth 10 -not -path './.git*' -not -path '*/_*' -not -path './vendor*' -type d); do
    if ls $dir/*.go &> /dev/null; then
        go test -covermode=${covermode} -coverprofile=$dir/profile.tmp $dir
        if [ -f $dir/profile.tmp ]; then
            tail -n +2 $dir/profile.tmp >> ${output}
            rm $dir/profile.tmp
        fi
    fi
done

go tool cover -func ${output}