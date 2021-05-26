#!/bin/bash

echo "Linting managers.go file"

inited=$(grep "return New" x/thorchain/managers.go |  awk '{print $2}' | awk -F "(" '{print $1}')
created=$(grep --exclude "*_test.go" "func New" x/thorchain/manager_* | awk '{print $2}' | awk -F "(" '{print $1}')
missing=$(echo -e "$inited\n$created" | grep -Ev 'Dummy|Helper|NewStoreMgr' | sort -n | uniq -u)
echo $missing

[ -z "$missing" ] && echo "OK" && exit 0

[[ ! -z "$missing" ]] && echo "Not OK" && exit 1
