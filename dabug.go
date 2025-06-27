package dabug

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

// Utility for printing multi or single line statements to aid
// tracking execution.

// Add easy log for variables
// Add single line quick logs that do not require flush
// switch context to be map, sort the keys

type Dabugger struct {
	// lines contains lines waiting to be flushed
	lines      []*line
	linesMutex sync.Mutex
	contexts   []*dcontext
	writer     io.Writer
	linePrefix string
	autoFlush  bool
	stackSkips int
}

type dcontext struct {
	key   string
	value string
}

type line struct {
	msg    string
	src    *source
	prefix string
}

type source struct {
	File     string
	Function string
	Line     int
}

func (s source) String() string {
	return fmt.Sprintf("%s:%d", s.File, s.Line)
}

var (
	defDabugger = &Dabugger{}
	sectionBeg  = "-----"
	sectionEnd  = "====="
	// baseDir   string
)

func init() {
	defDabugger = New()
	defDabugger.autoFlush = true
	defDabugger.linePrefix = "DABUG: "
	defDabugger.stackSkips++
}

// New creates a new dabugger with autoflush disabled
func New() *Dabugger {
	prefix := ""
	if defDabugger != nil && defDabugger.linePrefix != "" {
		// Inherit the line prefix from the default debugger
		prefix = defDabugger.linePrefix
	}

	return &Dabugger{
		writer:     os.Stdout,
		autoFlush:  false,
		stackSkips: 4,
		linePrefix: prefix,
	}
}

type contextKey int

const (
	dabuggerContextKey contextKey = iota
)

func (d *Dabugger) StoreInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, dabuggerContextKey, d)
}

func FromContext(ctx context.Context) (*Dabugger, bool) {
	v, ok := ctx.Value(dabuggerContextKey).(*Dabugger)
	return v, ok
}

// Stack dumps the last num lines of the stack trace, set num to any
// negative number to print the full stack.
func Stack(num int) {
	defDabugger.Stack(num)
}

func (d *Dabugger) Stack(num int) {
	stackLines := strings.Split(string(debug.Stack()), "\n")
	for i, line := range stackLines {
		d.appendMsg(line)
		if num > 0 && i+1 >= num {
			break
		}
	}
}

// Writer sets the writer to print statements to.
func Writer(writer io.Writer) {
	defDabugger.Writer(writer)
}

func (d *Dabugger) Writer(writer io.Writer) {
	d.writer = writer
}

// LinePrefix sets a prefix to prepend to every line printed.
func LinePrefix(prefix string) {
	defDabugger.LinePrefix(prefix)
}

func (d *Dabugger) LinePrefix(prefix string) {
	d.linePrefix = prefix
}

func AutoFlush(flush bool) {
	defDabugger.AutoFlush(flush)
}

func (d *Dabugger) AutoFlush(flush bool) {
	d.autoFlush = flush
	if len(defDabugger.lines) > 0 {
		d.Flush()
	}
}

func Msg(format string, v ...any) {
	defDabugger.Msg(format, v...)
}

func (d *Dabugger) Msg(format string, v ...any) {
	d.appendMsg(fmt.Sprintf(format, v...))
}

func Here() {
	defDabugger.Here()
}

func (d *Dabugger) Here() {
	d.appendEmpty()
}

// Objs will append a line to the logger with things printed.
func Objs(things ...any) {
	defDabugger.Objs(things...)
}

func (d *Dabugger) Objs(things ...any) {
	var msgs []string
	for i, t := range things {
		msg := fmt.Sprintf("[%d] %#v", i, t)
		msgs = append(msgs, msg)
	}
	d.appendMsg(strings.Join(msgs, ", "))
}

// AddContext adds a key/value pair that will be prepended to log
func AddContext(key, value string) {
	defDabugger.AddContext(key, value)
}

func (d *Dabugger) AddContext(key, value string) {
	d.contexts = append(d.contexts, &dcontext{key, value})
}

func RemoveContext(key string) {
	defDabugger.RemoveContext(key)
}

func (d *Dabugger) RemoveContext(key string) {
	newContexts := []*dcontext{}
	for _, c := range d.contexts {
		if c.key != key {
			newContexts = append(newContexts, c)
		}
	}

	d.contexts = newContexts
}

func RemoveAllContext() {
	defDabugger.RemoveAllContext()
}

func (d *Dabugger) RemoveAllContext() {
	clear(d.contexts)
	d.contexts = nil
}

func RemoveTopContext() {
	defDabugger.RemoveTopContext()
}

func (d *Dabugger) RemoveTopContext() {
	d.contexts = d.contexts[:len(d.contexts)-1]
}

func Flush() {
	defDabugger.Flush()
}

func (d *Dabugger) Flush() {
	d.linesMutex.Lock()
	defer d.linesMutex.Unlock()

	if len(d.lines) == 0 {
		// Nothing to do
		return
	}

	// preprocess line prefix len so that all messages are aligned
	maxPrefixLen := -1
	for _, l := range d.lines {
		maxPrefixLen = max(maxPrefixLen, len(l.prefix))
	}

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("%s%s\n", d.linePrefix, sectionBeg))

	lFmt := fmt.Sprintf("%%-%ds", maxPrefixLen)
	for _, l := range d.lines {
		sb.WriteString(lineStr(lFmt, l) + "\n")
	}

	sb.WriteString(fmt.Sprintf("%s%s\n", d.linePrefix, sectionEnd))

	fmt.Fprint(d.writer, sb.String())

	d.clearLines()
}

func (d *Dabugger) flushLine(l *line) {
	msg := lineStr("%s", l)
	fmt.Fprintf(d.writer, "%s\n", msg)
}

func lineStr(lFmt string, l *line) string {
	var msg string

	if len(l.msg) == 0 {
		msg = fmt.Sprintf(lFmt, l.prefix)
	} else {
		suffix := "- %s"
		msg = fmt.Sprintf(lFmt+suffix, l.prefix, l.msg)
	}

	return msg
}

func (d *Dabugger) appendLine(line *line) {
	d.genPrefix(line)

	if d.autoFlush {
		d.flushLine(line)
		return
	}

	d.linesMutex.Lock()
	defer d.linesMutex.Unlock()

	d.lines = append(d.lines, line)
}

func (d *Dabugger) appendEmpty() {
	line := &line{src: d.getSource()}
	d.appendLine(line)
}

func (d *Dabugger) appendMsg(msg string) {
	line := &line{
		msg: msg,
		src: d.getSource(),
	}
	d.appendLine(line)
}

func (d *Dabugger) genPrefix(line *line) {
	c := strings.Builder{}
	if len(d.contexts) > 0 {
		c.WriteString(" (")
	}
	for i := 0; i < len(d.contexts); i++ {
		context := d.contexts[i]
		kv := fmt.Sprintf("%s:%s", context.key, context.value)

		if i > 0 {
			c.WriteString(", ")
		}
		c.WriteString(kv)
	}
	if len(d.contexts) > 0 {
		c.WriteString(")")
	}

	p := fmt.Sprintf("%s%s%s ", d.linePrefix, line.src, c.String())

	line.prefix = p
}

func (d *Dabugger) getSource() *source {
	var pc uintptr
	var pcs [1]uintptr

	// skip
	// 1. runtime.Callers
	// 2. getSource
	// 3. appendLine
	// 4. Msg, Objs, etc.
	runtime.Callers(d.stackSkips, pcs[:])
	pc = pcs[0]

	fs := runtime.CallersFrames([]uintptr{pc})
	f, _ := fs.Next()

	_, fpath, _, _ := runtime.Caller(3)

	var file string
	file = strings.TrimPrefix(f.File, filepath.Dir(fpath))
	file = strings.TrimPrefix(file, "/")

	return &source{
		File:     file,
		Function: f.Function,
		Line:     f.Line,
	}
}

func (d *Dabugger) clearLines() {
	d.lines = []*line{}
}
