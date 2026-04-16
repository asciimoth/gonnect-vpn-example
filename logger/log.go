package logger

import "log"

// Static type assertion
var _ Logger = &log.Logger{}

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
