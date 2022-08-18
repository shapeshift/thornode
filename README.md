# THORChain

## Fork Maintenance

The gitlab repository does not track the generated protobuf `pb.go` files for security reasons.

In order to use `gitlab.com/thorchain/thornode` as a golang dependency, a fork needs to be maintained and updated for and supported versions.

### Steps to support a new release:

1. Add upstream repository
	```sh
	git remote add upstream https://gitlab.com/thorchain/thornode.git
	```

1. Fetch release branch
	```sh
	git fetch upstream release-1.95.0
	```

1. Remove cached terraform artifacts that blow out the github upload limit
	```sh
	git filter-branch -f --index-filter 'git rm --cached -r --ignore-unmatch infra/.terraform/'
	```

1. Generate protobuf files
	```sh
	make protob
	```

1. Update `.gitignore` to track `pb.go` files
	```sh
	sed -i '/*.pb.go/d' .gitignore
	```

1. Add and commit files
	```sh
	git add .
	git commit -m "chore(release): v1.95.0 (generated protobuf)"
	```

1. Tag release version
	```sh
	git tag v1.95.0
	```

1. Push release branch and tag
	```sh
	git push origin release-1.95.0
	git push origin v1.95.0
	```

### Steps to use forked dependency in your golang project:

1. Add `gitlab.com/thorchain/thornode` dependency
	```sh
	go get gitlab.com/thorchain/thornode
	```

1. Replace dependency with forked version
	```sh
	go mod edit -replace gitlab.com/thorchain/thornode=github.com/shapeshift/thornode@v1.95.0
	```

1. Tidy go modules as needed
	```sh
	go mod tidy
	```
