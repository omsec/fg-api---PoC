package helpers

import "runtime"

// https://stackoverflow.com/questions/25927660/how-to-get-the-current-function-name

// FuncName returns the name of the function only (easier calling in error handlers)
func FuncName() string {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		return "?"
	}

	fn := runtime.FuncForPC(pc)
	return fn.Name()
}

// Trace is used for reflection
// eg. embedded into error wrappers or use in loggers
func Trace() (string, int, string) {
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		return "?", 0, "?"
	}

	fn := runtime.FuncForPC(pc)
	return file, line, fn.Name()
}
