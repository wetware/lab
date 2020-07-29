WWPATH := $(GOPATH)/src/github.com/wetware/ww

all: clean install

install: link
	@-mkdir ./extra
	@ln -s $(WWPATH) ./extra/ww

link:
	@go mod tidy
	@go mod edit -replace github.com/wetware/ww=$(PWD)/extra/ww

clean:
	@rm -rf ./extra
