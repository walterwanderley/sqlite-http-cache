
.PRONY: builder
builder:
	docker build -f Dockerfile-builder -t builder-sqlite-http-cache .

.PHONY: snapshot
snapshot: builder
	docker run -e GITHUB_TOKEN=${GITHUB_TOKEN} builder-sqlite-http-cache goreleaser release --clean --snapshot --skip publish

.PHONY: release
release: builder
	docker run -e GITHUB_TOKEN=${GITHUB_TOKEN} builder-sqlite-http-cache goreleaser release --clean