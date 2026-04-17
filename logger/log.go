package logger

import (
	"log"
	"testing"
)

// Static type assertion
var (
	_ Logger = &log.Logger{}
	_ Logger = &TestingLogger{}
)

type Logger interface {
	Print(v ...any)
	Println(v ...any)
	Printf(format string, v ...any)

	Fatal(v ...any)
	Fatalf(format string, v ...any)
	Fatalln(v ...any)

	Panic(v ...any)
	Panicf(format string, v ...any)
	Panicln(v ...any)
}

type TestingLogger struct {
	*testing.T
}

func (t *TestingLogger) Print(v ...any) {
	t.T.Log(v...) // nolint
}

func (t *TestingLogger) Println(v ...any) {
	t.T.Log(v...) // nolint
}

func (t *TestingLogger) Printf(format string, v ...any) {
	t.T.Logf(format, v...) // nolint
}

func (t *TestingLogger) Fatal(v ...any) {
	t.T.Error(v...) // nolint
}

func (t *TestingLogger) Fatalln(v ...any) {
	t.T.Error(v...) // nolint
}

func (t *TestingLogger) Fatalf(format string, v ...any) {
	t.T.Errorf(format, v...) // nolint
}

func (t *TestingLogger) Panic(v ...any) {
	t.T.Fatal(v...) // nolint
}

func (t *TestingLogger) Panicln(v ...any) {
	t.T.Fatal(v...) // nolint
}

func (t *TestingLogger) Panicf(format string, v ...any) {
	t.T.Fatalf(format, v...) // nolint
}
