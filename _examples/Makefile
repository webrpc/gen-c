WEBRPC_GEN_VERSION := v0.37.1
ROOT := ..
GEN := go run -ldflags="-X github.com/webrpc/webrpc.VERSION=$(WEBRPC_GEN_VERSION)" github.com/webrpc/webrpc/cmd/webrpc-gen@$(WEBRPC_GEN_VERSION)

.PHONY: generate diff

generate:
	$(GEN) -schema=smoke/example.ridl -target=$(ROOT) -emit=header -out=smoke/example.gen.h
	$(GEN) -schema=smoke/example.ridl -target=$(ROOT) -emit=impl -header=example.gen.h -out=smoke/example.gen.c

diff:
	git diff --exit-code -- smoke/example.gen.h smoke/example.gen.c
