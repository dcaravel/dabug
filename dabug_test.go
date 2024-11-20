package dabug

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHere(t *testing.T) {
	sb := &strings.Builder{}

	AutoFlush(true)
	Writer(sb)
	Here()

	parts := strings.Split(sb.String(), "\n")
	require.Len(t, parts, 2)
	assert.Contains(t, parts[0], "dabug_test")
	assert.Zero(t, parts[1])
}

func TestMsg(t *testing.T) {
	sb := &strings.Builder{}

	AutoFlush(true)
	Writer(sb)
	Msg("msg")

	parts := strings.Split(sb.String(), "\n")
	require.Len(t, parts, 2)
	assert.Contains(t, parts[0], "dabug_test")
	assert.Contains(t, parts[0], "msg")
	assert.Zero(t, parts[1])
}

func TestObjs(t *testing.T) {
	type person struct {
		name string
		loc  string
	}
	p1 := person{"dave", "earth"}
	p2 := &person{"fred", "mars"}

	sb := &strings.Builder{}

	AutoFlush(true)
	Writer(sb)
	Objs(p1, p2)

	parts := strings.Split(sb.String(), "\n")
	require.Len(t, parts, 2)
	assert.Contains(t, parts[0], "dabug_test")
	assert.Contains(t, parts[0], "dave")
	assert.Contains(t, parts[0], "earth")
	assert.Contains(t, parts[0], "fred")
	assert.Contains(t, parts[0], "mars")
	assert.Zero(t, parts[1])
}

func TestFlush(t *testing.T) {
	sb := &strings.Builder{}

	AutoFlush(false)
	Writer(sb)
	Msg("msg")

	parts := strings.Split(sb.String(), "\n")
	require.Len(t, parts, 1)

	Flush()

	parts = strings.Split(sb.String(), "\n")
	require.Len(t, parts, 4)
	assert.Contains(t, parts[0], sectionBeg)
	assert.Contains(t, parts[1], "msg")
	assert.Contains(t, parts[2], sectionEnd)
	assert.Zero(t, parts[3])
}

func TestPrefix(t *testing.T) {
	sb := &strings.Builder{}

	AutoFlush(true)
	Writer(sb)
	LinePrefix("prefix")
	Msg("msg")

	parts := strings.Split(sb.String(), "\n")
	require.Len(t, parts, 2)
	assert.Contains(t, parts[0], "prefix")
	assert.Contains(t, parts[0], "msg")
	assert.Zero(t, parts[1])

	Msg("gsm")
	parts = strings.Split(sb.String(), "\n")
	require.Len(t, parts, 3)
	assert.Contains(t, parts[0], "prefix")
	assert.Contains(t, parts[0], "msg")
	assert.Contains(t, parts[1], "prefix")
	assert.Contains(t, parts[1], "gsm")
	assert.Zero(t, parts[2])
}

func TestContext(t *testing.T) {
	sb := &strings.Builder{}

	AutoFlush(true)
	Writer(sb)
	AddContext("hello", "world")
	Msg("msg")

	fmt.Print(sb.String())

	sb = &strings.Builder{}
	Writer(sb)
	AddContext("good", "bye")
	Msg("msg")
	Msg("msg2")
	RemoveTopContext()
	Msg("msg3")
	AddContext("sup", "gee")
	Msg("msg4")
	RemoveContext("hello")
	fmt.Print(sb.String())
}
