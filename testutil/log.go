package testutil

import (
	log "github.com/lthibault/log/pkg"
	"go.uber.org/zap"

	"github.com/testground/sdk-go/runtime"
)

/*
	log.go contains utilities for interop between go.uber.org/zap (used by Testground)
	and github.com/lthibault/log (used by wetware).
*/

var _ log.Logger = (*zapper)(nil)

type zapper struct {
	*zap.SugaredLogger
}

// ZapLogger wraps a *zap.SugaredLogger such that it satisfies ww's standard log.Logger
// interface.
//
// Trace messages will be reported at the debug level.
func ZapLogger(runenv *runtime.RunEnv) log.Logger {
	return zapper{runenv.SLogger()}
}

func (z zapper) Fatalln(v ...interface{}) { z.Fatal(v...) }

func (z zapper) Trace(v ...interface{})            { z.Debug(v...) }
func (z zapper) Tracef(s string, v ...interface{}) { z.Debugf(s, v...) }
func (z zapper) Traceln(v ...interface{})          { z.Debugln(v...) }

func (z zapper) Debugln(v ...interface{}) { z.Debug(v...) }

func (z zapper) Infoln(v ...interface{}) { z.Info(v...) }

func (z zapper) Warnln(v ...interface{}) { z.Warn(v...) }

func (z zapper) Errorln(v ...interface{}) { z.Error(v...) }

func (z zapper) WithError(err error) log.Logger               { return zapper{z.With("error", err)} }
func (z zapper) WithField(s string, v interface{}) log.Logger { return zapper{z.With(s, v)} }
func (z zapper) WithFields(f log.F) log.Logger {
	fs := make([]interface{}, 0, len(f)*2)
	for k, v := range f {
		fs = append(fs, k)
		fs = append(fs, v)
	}

	return zapper{z.With(fs...)}
}
