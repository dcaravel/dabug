package dabug

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
	contexts   []*context
	writer     io.Writer
	linePrefix string
	autoFlush  bool
	stackSkips int
}

type context struct {
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
	defDabugger.stackSkips++
}

func New() *Dabugger {
	prefix := "DABUG: "
	if defDabugger != nil {
		prefix = defDabugger.linePrefix
	}

	return &Dabugger{
		writer:     os.Stdout,
		autoFlush:  true,
		stackSkips: 4,
		linePrefix: prefix,
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

func (l *Dabugger) Msg(format string, v ...any) {
	l.appendMsg(fmt.Sprintf(format, v...))
}

func Here() {
	defDabugger.Here()
}

func (l *Dabugger) Here() {
	l.appendEmpty()
}

// Objs will append a line to the logger with things printed.
func Objs(things ...any) {
	defDabugger.Objs(things...)
}

func (l *Dabugger) Objs(things ...any) {
	var msgs []string
	for i, t := range things {
		msg := fmt.Sprintf("[%d] %#v", i, t)
		msgs = append(msgs, msg)
	}
	l.appendMsg(strings.Join(msgs, ", "))
}

// AddContext adds a key/value pair that will be prepended to log
func AddContext(key, value string) {
	defDabugger.AddContext(key, value)
}

func (l *Dabugger) AddContext(key, value string) {
	l.contexts = append(l.contexts, &context{key, value})
}

func RemoveContext(key string) {
	defDabugger.RemoveContext(key)
}

func (l *Dabugger) RemoveContext(key string) {
	newContexts := []*context{}
	for _, c := range l.contexts {
		if c.key != key {
			newContexts = append(newContexts, c)
		}
	}

	l.contexts = newContexts
}

func RemoveAllContext() {
	defDabugger.RemoveAllContext()
}

func (l *Dabugger) RemoveAllContext() {
	clear(l.contexts)
	l.contexts = nil
}

func RemoveTopContext() {
	defDabugger.RemoveTopContext()
}

func (l *Dabugger) RemoveTopContext() {
	l.contexts = l.contexts[:len(l.contexts)-1]
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

func (l *Dabugger) clearLines() {
	l.lines = []*line{}
}
