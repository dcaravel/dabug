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

type logger struct {
	// lines contains lines waiting to be flushed
	lines      []*line
	linesMutex sync.Mutex
	contexts   []*context
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
	defLogger     = &logger{}
	defWriter     io.Writer
	defLinePrefix = ""
	autoflush     = true
	sectionBeg    = "-----"
	sectionEnd    = "====="
	// baseDir   string
)

func init() {
	defWriter = os.Stdout
	// baseDir = filepath.Dir(os.Args[0])
}

// Writer sets the writer to print statements to.
func Writer(writer io.Writer) {
	defWriter = writer
}

// LinePrefix sets a prefix to prepend to every line printed.
func LinePrefix(prefix string) {
	defLinePrefix = prefix
}

func AutoFlush(flush bool) {
	autoflush = flush
	if len(defLogger.lines) > 0 {
		Flush()
	}
}

func Msg(msg string) {
	defLogger.appendMsg(msg)
}

func Here() {
	defLogger.appendEmpty()
}

// Objs will append a line to the logger with things printed.
func Objs(things ...any) {
	var msgs []string
	for i, t := range things {
		msg := fmt.Sprintf("[%d] %#v", i, t)
		msgs = append(msgs, msg)
	}
	defLogger.appendMsg(strings.Join(msgs, ", "))
}

// AddContext adds a key/value pair that will be prepended to log
func AddContext(key, value string) {
	defLogger.AddContext(key, value)
}

func (l *logger) AddContext(key, value string) {
	l.contexts = append(l.contexts, &context{key, value})
}

func RemoveContext(key string) {
	defLogger.RemoveContext(key)
}

func (l *logger) RemoveContext(key string) {

}

func RemoveAllContext() {

}

func RemoveTopContext() {
	defLogger.removeTopContext()
}

func (l *logger) removeTopContext() {
	l.contexts = l.contexts[:len(l.contexts)-1]
}

func Flush() {
	defLogger.linesMutex.Lock()
	defer defLogger.linesMutex.Unlock()

	if len(defLogger.lines) == 0 {
		// Nothing to do
		return
	}

	// preprocess line prefix len so that all messages are aligned
	maxPrefixLen := -1
	for _, l := range defLogger.lines {
		maxPrefixLen = max(maxPrefixLen, len(l.prefix))
	}

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("%s%s\n", defLinePrefix, sectionBeg))

	lFmt := fmt.Sprintf("%%-%ds", maxPrefixLen)
	for _, l := range defLogger.lines {
		sb.WriteString(lineStr(lFmt, l) + "\n")
	}

	sb.WriteString(fmt.Sprintf("%s%s\n", defLinePrefix, sectionEnd))

	fmt.Fprint(defWriter, sb.String())

	clear()
}

func flushLine(l *line) {
	msg := lineStr("%s", l)
	fmt.Fprintf(defWriter, "%s\n", msg)
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

func (l *logger) appendLine(line *line) {
	l.genPrefix(line)

	if autoflush {
		flushLine(line)
		return
	}

	defLogger.linesMutex.Lock()
	defer defLogger.linesMutex.Unlock()

	defLogger.lines = append(defLogger.lines, line)
}

func (l *logger) appendEmpty() {
	line := &line{src: getSource()}
	l.appendLine(line)
}

func (l *logger) appendMsg(msg string) {
	line := &line{
		msg: msg,
		src: getSource(),
	}
	l.appendLine(line)
}

func (l *logger) genPrefix(line *line) {
	c := strings.Builder{}
	if len(l.contexts) > 0 {
		c.WriteString(" (")
	}
	for i := 0; i < len(l.contexts); i++ {
		context := l.contexts[i]
		kv := fmt.Sprintf("%s:%s", context.key, context.value)

		if i > 0 {
			c.WriteString(", ")
		}
		c.WriteString(kv)
	}
	if len(l.contexts) > 0 {
		c.WriteString(")")
	}

	p := fmt.Sprintf("%s%s%s ", defLinePrefix, line.src, c.String())

	line.prefix = p
}

func getSource() *source {
	var pc uintptr
	var pcs [1]uintptr

	// skip
	// 1. runtime.Callers
	// 2. getSource
	// 3. appendLine
	// 4. Msg, Objs, etc.
	runtime.Callers(4, pcs[:])
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

func clear() {
	defLogger.lines = []*line{}
}
