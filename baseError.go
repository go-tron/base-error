package baseError

import (
	"fmt"
	"github.com/pkg/errors"
	"io"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

var sourceDir string

func init() {
	_, file, _, _ := runtime.Caller(0)
	// compatible solution to get gorm source directory with various operating systems
	sourceDir = getSourceDir(file)
}
func getSourceDir(file string) string {
	dir := filepath.Dir(file)
	dir = filepath.Dir(dir)

	s := filepath.Dir(dir)
	if filepath.Base(s) != "eioos.com" {
		s = dir
	}
	return filepath.ToSlash(s) + "/"
}

func IsSystem(err error) bool {
	return reflect.TypeOf(err).String() == "*baseError.Error" && !err.(*Error).System
}

type Error struct {
	Code   string `json:"code"`
	Msg    string `json:"msg"`
	System bool   `json:"-"`
	Chain  string `json:"-"`
	cause  error  `json:"-"`
	*stack
}

func (b *Error) WithSystem() *Error {
	b.System = true
	return b
}

func (b *Error) WithChain(chain ...string) *Error {
	b.Chain = strings.Join(chain, "<-")
	return b
}

func (b *Error) Error() string {
	return fmt.Sprintf("[%s] %s", b.Code, b.Msg)
}

func (b *Error) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			io.WriteString(s, b.Error())
			if b.stack != nil {
				b.stack.Format(s, verb)
			}
			if b.cause != nil {
				fmt.Fprintf(s, "\n---cause---\n%+v", b.cause)
			}
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, b.Error())
	case 'q':
		fmt.Fprintf(s, "%q", b.Error())
	}
}

func (b *Error) Stack() *stack {
	return b.stack
}

func (b *Error) Cause() error {
	return b.cause
}

func New(code string, msg string) *Error {
	return &Error{Code: code, Msg: msg}
}

func NewStack(code string, msg string, depth int) *Error {
	if depth == 0 {
		depth = 1
	}
	return &Error{Code: code, Msg: msg, stack: Callers(3, depth)}
}

func System(code string, msg string) *Error {
	return &Error{Code: code, Msg: msg, System: true}
}

func SystemStack(code string, msg string, depth int) *Error {
	if depth == 0 {
		depth = 1
	}
	return &Error{Code: code, Msg: msg, System: true, stack: Callers(3, depth)}
}

func factoryFormat(arg ...string) (string, func(message ...interface{}) string) {
	if len(arg) == 0 {
		panic("ErrorFactory至少包含一个参数code")
	}
	if len(arg) > 2 {
		panic("ErrorFactory最多有两个参数code,msg")
	}

	var (
		code string
		msg  string
	)
	if len(arg) == 1 {
		code = arg[0]
		msg = "{}"
	} else {
		code = arg[0]
		msg = arg[1]
	}

	if strings.Contains(msg, "{}") {
		msg = strings.ReplaceAll(msg, "{}", "%v")
	}
	c := strings.Count(msg, "%v")
	return code, func(message ...interface{}) string {
		if len(message) < c {
			for {
				if len(message) == c {
					break
				}
				message = append(message, "")
			}
		}
		return fmt.Sprintf(msg, message...)
	}
}

func Factory(arg ...string) func(...interface{}) *Error {
	code, formatter := factoryFormat(arg...)
	return func(message ...interface{}) *Error {
		fmtMsg := formatter(message...)
		return &Error{Code: code, Msg: fmtMsg}
	}
}

func FactoryStack(depth int, arg ...string) func(...interface{}) *Error {
	if depth == 0 {
		depth = 1
	}
	code, formatter := factoryFormat(arg...)
	return func(message ...interface{}) *Error {
		fmtMsg := formatter(message...)
		return &Error{Code: code, Msg: fmtMsg, stack: Callers(3, depth)}
	}
}

func SystemFactory(arg ...string) func(...interface{}) *Error {
	code, formatter := factoryFormat(arg...)
	return func(message ...interface{}) *Error {
		fmtMsg := formatter(message...)
		return &Error{Code: code, Msg: fmtMsg, System: true}
	}
}

func SystemFactoryStack(depth int, arg ...string) func(...interface{}) *Error {
	if depth == 0 {
		depth = 1
	}
	code, formatter := factoryFormat(arg...)
	return func(message ...interface{}) *Error {
		fmtMsg := formatter(message...)
		return &Error{Code: code, Msg: fmtMsg, System: true, stack: Callers(3, depth)}
	}
}

func Wrap(code string, err error) *Error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Msg: err.Error(), System: true, cause: err}
}

func WrapStack(code string, err error, depth int) *Error {
	if err == nil {
		return nil
	}
	if depth == 0 {
		depth = 1
	}
	return &Error{Code: code, Msg: err.Error(), System: true, cause: err, stack: Callers(3, depth)}
}

func WrapFactory(code string) func(err error) *Error {
	return func(err error) *Error {
		return Wrap(code, err)
	}
}

func WrapFactoryStack(depth int, code string) func(err error) *Error {
	if depth == 0 {
		depth = 1
	}
	return func(err error) *Error {
		return WrapStack(code, err, depth)
	}
}

type stack []uintptr

func (s *stack) Format(st fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case st.Flag('+'):
			//for _, pc := range *s {
			//	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
			//	if frame.PC != 0 {
			//		fmt.Fprintf(st, "\n%s:%d", frame.File, frame.Line)
			//	}
			//}
			for _, pc := range *s {
				f := errors.Frame(pc)
				fmt.Fprintf(st, "\n%+v", f)
			}
		}
	}
}
func (s *stack) StackTrace() errors.StackTrace {
	f := make([]errors.Frame, len(*s))
	for i := 0; i < len(f); i++ {
		f[i] = errors.Frame((*s)[i])
	}
	return f
}

func Callers(skip int, depth int) *stack {
	var s = skip
	for i := skip; i < 15; i++ {
		_, file, _, ok := runtime.Caller(i)
		if ok && (!strings.HasPrefix(file, sourceDir) || strings.HasSuffix(file, "_test.go")) {
			s = i + 1
			break
		}
	}
	pcs := make([]uintptr, depth)
	n := runtime.Callers(s, pcs[:])
	var st stack = pcs[0:n]
	return &st
}

func WithStack(err error, depth int) error {
	if err == nil {
		return nil
	}
	return &withStack{
		err,
		Callers(6, depth),
	}
}

type withStack struct {
	error
	*stack
}

func (w *withStack) Cause() error { return w.error }

func (w *withStack) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v", w.Cause())
			w.stack.Format(s, verb)
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, w.Error())
	case 'q':
		fmt.Fprintf(s, "%q", w.Error())
	}
}
