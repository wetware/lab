all: clean link

link:
	@go mod edit -replace github.com/wetware/ww=$(GOPATH)/src/github.com/wetware/ww

clean:
	@go mod edit -dropreplace github.com/wetware/ww
