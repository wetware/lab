ifeq ($(origin TESTGROUND_HOME), undefined)
	TESTGROUND_HOME := $(HOME)/testground
endif

all: clean link

clean: unlink

link:
	@ln -s $(CURDIR) $(TESTGROUND_HOME)/plans/casm

unlink:
	@rm -f $(TESTGROUND_HOME)/plans/casm
