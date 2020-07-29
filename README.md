# labtests

Simulation and integration tests for Wetware

This directory contains test-plans for use with [Testground](https://github.com/testground/testground).

These are cluster-level integration tests and simulations for the distributed protocols used by wetware.

## Dependencies

- [Testground](https://github.com/testground/testground)
- [Docker](https://www.docker.com/)

## Usage

### Initial setup

Estimated time:  about 5 minutes

1. Run `testground daemon`.  This will create a `$TESTGROUND_HOME` directory if it does not already exist (by default `$HOME/testground`)
2. Link this directory to your testground home directory with `ln -s <lab root>/test-plans <testground home>/plans/ww`.  Make sure you replace the root paths match those on your system.
3. Run `testground plan list` and check that the `ww` plan appears.

### Running test cases

TODO
