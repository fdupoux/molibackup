SHELL = bash
VERSION := $(file < VERSION)
BUILDOPT := -trimpath -ldflags="-buildid= -w" -buildvcs=false

run:
	@echo "Version is '$(VERSION)'"
	CGO_ENABLED=0 go run *.go

fmt:
	go fmt

vet:
	go vet

clean:
	rm -rf .cache molibackup molibackup-*

goinfo:
	go version

modtidy:
	CGO_ENABLED=0 go mod tidy -v

build:
	CGO_ENABLED=0 go build $(BUILDOPT) -o molibackup

packages:
	(cd molibackup-$(VERSION)-linux-amd64 && sha256sum -b molibackup-$(VERSION)-linux-amd64 > molibackup-$(VERSION)-linux-amd64.sha256)
	tar cfz molibackup-$(VERSION)-linux-amd64.tar.gz molibackup-$(VERSION)-linux-amd64
	(cd molibackup-$(VERSION)-linux-arm64 && sha256sum -b molibackup-$(VERSION)-linux-arm64 > molibackup-$(VERSION)-linux-arm64.sha256)
	tar cfz molibackup-$(VERSION)-linux-arm64.tar.gz molibackup-$(VERSION)-linux-arm64

release: clean
	mkdir -p molibackup-$(VERSION)-linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILDOPT) -o molibackup-$(VERSION)-linux-amd64/molibackup-$(VERSION)-linux-amd64
	mkdir -p molibackup-$(VERSION)-linux-arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(BUILDOPT) -o molibackup-$(VERSION)-linux-arm64/molibackup-$(VERSION)-linux-arm64

docker-goinfo:
	./docker.sh make goinfo

docker-modtidy:
	./docker.sh make modtidy

docker-build:
	./docker.sh make build

docker-release:
	./docker.sh make release
