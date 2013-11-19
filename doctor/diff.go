// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains an implementation of the greedy longest common
// subsequence/shortest edit script (LCS/SES) algorithm described in
// Eugene W. Myers, "An O(ND) Difference Algorithm and Its Variations"
//
// It also contains support for creating unified diffs (i.e., patch files).
// The unified diff format is documented in the POSIX standard (IEEE 1003.1),
// "diff - compare two files", section: "Diff -u or -U Output Format"
// http://pubs.opengroup.org/onlinepubs/9699919799/utilities/diff.html

// Contributors: Jeff Overbey

package doctor

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Diff creates an EditSet containing the minimum number of line additions and
// deletions necessary to change a into b.  Typically, both a and b will be
// slices containing \n-terminated lines of a larger string, although it is
// also possible compute character-by-character diffs by splitting a string on
// UTF-8 boundaries.  The resulting edits are keyed by the given filename, and
// the resulting EditSet is constructed so that it can be applied to the string
// strings.Join(a, "").
//
// Every edit in the resulting EditSet starts at an offset corresponding to the
// first character on a line.  Every edit in the EditSet is either (1) a
// deletion, i.e., its length is the length of the current line and its
// replacement text is the empty string, or (2) an addition, i.e., its length
// is 0 and its replacement text is a single line to insert.
func Diff(filename string, a []string, b []string) EditSet {
	n := len(a)
	m := len(b)
	max := m + n
	if n == 0 && m == 0 {
		return NewEditSet()
	} else if n == 0 {
		result := NewEditSet()
		result.Add(filename, OffsetLength{0, 0}, strings.Join(b, ""))
		return result
	} else if m == 0 {
		result := NewEditSet()
		result.Add(filename, OffsetLength{0, len(strings.Join(a, ""))}, "")
		return result
	}
	vs := make([][]int, 0, max)
	v := make([]int, 2*max, 2*max)
	offset := max
	v[offset+1] = 0
	for d := 0; d <= max; d++ {
		for k := -d; k <= d; k += 2 {
			var x, y int
			var vert bool
			if k == -d || k != d &&
				abs(v[offset+k-1]) < abs(v[offset+k+1]) {
				x = abs(v[offset+k+1])
				vert = false
			} else {
				x = abs(v[offset+k-1]) + 1
				vert = true
			}
			y = x - k
			for x < n && y < m && a[x] == b[y] {
				x, y = x+1, y+1
			}
			if vert {
				v[offset+k] = -x
			} else {
				v[offset+k] = x
			}
			if x >= n && y >= m {
				// length of SES is D
				vs = append(vs, v)
				return constructEditSet(filename, a, b, vs)
			}
		}
		v_copy := make([]int, len(v))
		copy(v_copy, v)
		vs = append(vs, v_copy)
	}
	panic("Length of SES longer than max (impossible)")
}

// Abs returns the absolute value of an integer
func abs(n int) int {
	if n < 0 {
		return -n
	} else {
		return n
	}
}

// ConstructEditSet is a utility method invoked by Diff upon completion.  It
// uses the matrix vs (computed by Diff) to compute a sequence of deletions and
// additions.
func constructEditSet(filename string, a []string, b []string, vs [][]int) EditSet {
	n := len(a)
	m := len(b)
	max := m + n
	offset := max
	result := NewEditSet()
	k := n - m
	for len(vs) > 1 {
		v := vs[len(vs)-1]
		v_k := v[offset+k]
		x := abs(v_k)
		y := x - k

		vs = vs[:len(vs)-1]
		v = vs[len(vs)-1]
		if v_k > 0 {
			k++
		} else {
			k--
		}
		next_v_k := v[offset+k]
		next_x := abs(next_v_k)
		next_y := next_x - k

		if v_k > 0 {
			// Insert
			charsToCopy := y - next_y - 1
			insertOffset := x - charsToCopy
			ol := OffsetLength{offsetOfString(insertOffset, a), 0}
			copyOffset := y - charsToCopy - 1
			replaceWith := b[copyOffset : copyOffset+1]
			result.Add(filename, ol, strings.Join(replaceWith, ""))
		} else {
			// Delete
			charsToCopy := x - next_x - 1
			deleteOffset := x - charsToCopy - 1
			ol := OffsetLength{
				offsetOfString(deleteOffset, a),
				len(a[deleteOffset])}
			replaceWith := ""
			result.Add(filename, ol, replaceWith)
		}
	}
	return result
}

// OffsetOfString returns the byte offset of the substring ss[index] in the
// string strings.Join(ss, "")
func offsetOfString(index int, ss []string) int {
	result := 0
	for i := 0; i < index; i++ {
		result += len(ss[i])
	}
	return result
}

// -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// Number of leading/trailing context lines in a unified diff
const num_ctx_lines int = 3

// A Patch is an object representing a unified diff.  It can be created from an
// EditSet by invoking the CreatePatch method.
//
// Patch implements the EditSet interface, so a patch can be applied just as
// any other EditSet can.  However, patches are read-only; the Add method will
// always return an error.
type Patch struct {
	filename string
	hunks    []*hunk
}

// FIXME(jeff) produce correct output when no edits are applied
// FIXME(jeff) produce multi-file patches
// with one file, this doesn't quite match the intent of the EditSet interface

func (p *Patch) Edits() map[string][]edit {
	edits := []edit{}
	for _, hunk := range p.hunks {
		edits = append(edits, hunk.edits...)
	}
	return map[string][]edit{p.filename: edits}
}

func (p *Patch) Add(string, OffsetLength, string) error {
	return errors.New("Add cannot be called on Patch (read-only)")
}
func (p *Patch) ApplyTo(key string, in io.Reader, out io.Writer) error {
	panic("Not implemented")
}

func (p *Patch) ApplyToFile(filename string, out io.Writer) error {
	panic("Not implemented")
}

func (p *Patch) ApplyToString(key string, s string) (string, error) {
	panic("Not implemented")
}

func (p *Patch) CreatePatch(key string, in io.Reader) (*Patch, error) {
	return p, nil
}

func (p *Patch) String() string {
	var result bytes.Buffer
	p.Write(&result)
	return result.String()
}

// Add appends a hunk to this patch.  It is the caller's responsibility to
// ensure that hunks are added in the correct order.
func (p *Patch) add(hunk *hunk) {
	p.hunks = append(p.hunks, hunk)
}

// Write writes a unified diff to the given io.Writer.
func (p *Patch) Write(out io.Writer) error {
	writer := bufio.NewWriter(out)
	defer writer.Flush()
	fmt.Fprintf(writer, "--- %s\n+++ %s\n", p.filename, p.filename)
	lineOffset := 0
	for _, hunk := range p.hunks {
		adjust, err := writeUnifiedDiffHunk(hunk, lineOffset, writer)
		if err != nil {
			return err
		}
		lineOffset += adjust
	}
	return nil
}

// WriteUnifiedDiffHunk writes a single hunk in unified diff format.
func writeUnifiedDiffHunk(h *hunk, outputLineOffset int, out io.Writer) (int, error) {
	es := editSet{edits: map[string][]edit{"": h.edits}}
	var newTextBuffer bytes.Buffer
	hunk := h.hunk.String()
	err := es.ApplyTo("", strings.NewReader(hunk), &newTextBuffer)
	if err != nil {
		return 0, err
	}
	newText := newTextBuffer.String()

	origLines := strings.SplitAfter(hunk, "\n")
	newLines := strings.SplitAfter(newText, "\n")

	if _, err = fmt.Fprintf(out, "@@ -%d,%d +%d,%d @@\n",
		h.startLine, len(origLines),
		h.startLine+outputLineOffset, len(newLines)); err != nil {
		return 0, err
	}

	diff := Diff("", origLines, newLines)
	var sesIter *editIter
	switch ses := diff.(type) {
	case *editSet:
		sesIter = ses.newEditIter("")
	default:
		panic("Unreachable")
	}

	offset := 0
	for i, line := range origLines {
		if sesIter.edit() == nil || sesIter.edit().Offset > offset {
			fmt.Fprintf(out, " %s", origLines[i])
		} else {
			deleted := false
			for sesIter.edit() != nil && sesIter.edit().Offset == offset {
				edit := sesIter.edit()
				if edit.Length > 0 {
					fmt.Fprintf(out, "-%s", origLines[i])
					deleted = true
				} else {
					fmt.Fprintf(out, "+%s", edit.replacement)
				}
				sesIter.moveToNextEdit()
			}
			if !deleted {
				fmt.Fprintf(out, " %s", origLines[i])
			}
		}
		offset += len(line)
	}
	return len(newLines) - len(origLines), nil
}

// A hunk represents a single hunk in a unified diff.  A hunk consists of all
// of the edits that affect a particular region of a file.  Typically, a hunk
// is written with num_ctx_lines (3) lines of context preceding and following
// the hunk.  So, edits are grouped: if there are more than 6 lines between two
// edits, they should be in separate hunks.  Otherwise, the two edits should be
// in the same hunk.
type hunk struct {
	startOffset int          // Offset of this hunk in the original file
	startLine   int          // 1-based line number of this hunk in the orig file
	numLines    int          // Number of lines modified by this hunk
	hunk        bytes.Buffer // Affected bytes from the original file
	edits       []edit       // Edits to be applied to hunk
}

func (h *hunk) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Line: %d\nOffset: %d\n", h.startLine, h.startOffset)
	fmt.Fprintf(&buf, "Number of Lines: %d\n", h.numLines)
	fmt.Fprintf(&buf, "Original Text:\nvvvvv\n%s\n^^^^^\n", h.hunk.String())
	fmt.Fprintf(&buf, "Edits:\n")
	for _, edit := range h.edits {
		fmt.Fprintf(&buf, "%s\n", edit.String())
	}
	return buf.String()
}

// AddLine adds a single line of text to the hunk.
func (h *hunk) addLine(line string) {
	h.hunk.WriteString(line)
	h.numLines++
}

// AddEdit appends a single edit to the hunk.  It is the caller's
// responsibility to ensure that edits are added in sorted order.
func (h *hunk) addEdit(e *edit) {
	h.edits = append(h.edits, e.RelativeToOffset(h.startOffset))
}

// A lineRdr reads lines, one at a time, from an io.Reader, keeping track of
// the 0-based offset and 1-based line number of the line.  It also keeps
// track of the previous num_ctx_lines lines that were read.  (This is used to
// create leading context for a unified diff hunk.)
type lineRdr struct {
	reader          *bufio.Reader
	line            string
	lineOffset      int
	lineNum         int
	err             error
	leadingCtxLines []string
}

// NewLineRdr creates a new lineRdr that reads from the given io.Reader.
func newLineRdr(in io.Reader) *lineRdr {
	return &lineRdr{
		reader:          bufio.NewReader(in),
		line:            "",
		lineOffset:      0,
		lineNum:         0,
		leadingCtxLines: []string{},
	}
}

// ReadLine reads a single line from the wrapped io.Reader.  When the end of
// the input is reached, it returns io.EOF.
func (l *lineRdr) readLine() error {
	if l.lineNum > 0 {
		if len(l.leadingCtxLines) == num_ctx_lines {
			l.leadingCtxLines = l.leadingCtxLines[1:]
		}
		l.leadingCtxLines = append(l.leadingCtxLines, l.line)
	}
	l.lineOffset += len(l.line)
	l.lineNum++
	l.line, l.err = l.reader.ReadString('\n')
	return l.err
}

// Returns the 0-based offset of the first character on the line following the
// line that was read, or the length of the file if the end was reached.
func (l *lineRdr) offsetPastEnd() int {
	return l.lineOffset + len(l.line)
}

// Returns true iff the given edit adds characters at the beginning of this line
// without modifying or deleting any characters in the line.
func (l *lineRdr) editAddsToStart(e *edit) bool {
	if e == nil {
		return false
	} else {
		return e.Offset == l.lineOffset && e.Length == 0
	}
}

// Returns true iff the given edit adds characters to, modifies, or deletes
// characters from the line that was most recently read.
func (l *lineRdr) currentLineIsAffectedBy(e *edit) bool {
	if e == nil {
		return false
	} else {
		return e.Offset < l.offsetPastEnd() &&
			e.OffsetPastEnd() >= l.lineOffset
	}
}

// Returns true iff the given edit adds characters to, modifies, or deletes
// characters from the line following the line that was most recently read.
func (l *lineRdr) nextLineIsAffectedBy(e *edit) bool {
	if e == nil {
		return false
	} else {
		return e.OffsetPastEnd() >= l.offsetPastEnd()
	}
}

// StartHunk creats a new hunk, adding the current line and up to
// num_ctx_lines of leading context.
func startHunk(lr *lineRdr) *hunk {
	h := hunk{}
	h.startOffset = lr.lineOffset
	h.startLine = lr.lineNum
	h.numLines = 1

	for _, line := range lr.leadingCtxLines {
		h.startOffset -= len(line)
		h.startLine--
		h.numLines++
		h.hunk.WriteString(line)
	}

	h.hunk.WriteString(lr.line)
	return &h
}

// An iterator for []edit slices.
type editIter struct {
	edits     []edit
	nextIndex int
}

// Creates a new editIter with the first edit in the given file marked.
func (e *editSet) newEditIter(filename string) *editIter {
	return &editIter{e.edits[filename], 0}
}

// Edit returns the edit currently under the mark, or nil if no edits remain.
func (e *editIter) edit() *edit {
	if e.nextIndex >= len(e.edits) {
		return nil
	} else {
		return &e.edits[e.nextIndex]
	}
}

// MoveToNextEdit moves the mark to the next edit.
func (e *editIter) moveToNextEdit() {
	e.nextIndex++
}

// The CreatePatch method on editSet delegates to this method, which creates
// a Patch from an editSet.
func createPatch(e *editSet, key string, in io.Reader) (result *Patch, err error) {
	result = &Patch{filename: key}

	if len(e.edits) == 0 {
		return
	}

	const (
		HUNK_NOT_STARTED int = iota
		ADDING_TO_HUNK
		EDIT_ADDED_TO_HUNK
	)

	reader := newLineRdr(in)
	editIter := e.newEditIter(key)
	curState := HUNK_NOT_STARTED
	var hunk *hunk = nil
	var trailingCtxLines int

	for err = reader.readLine(); err == nil; err = reader.readLine() {
		switch curState {
		case HUNK_NOT_STARTED:
			if reader.currentLineIsAffectedBy(editIter.edit()) {
				hunk = startHunk(reader)
				last := addAllEditsOnCurLine(hunk, reader, editIter)
				if reader.nextLineIsAffectedBy(last) {
					curState = ADDING_TO_HUNK
				} else {
					if reader.editAddsToStart(last) {
						trailingCtxLines = 1
					} else {
						trailingCtxLines = 0
					}
					curState = EDIT_ADDED_TO_HUNK
				}
			} else {
				curState = HUNK_NOT_STARTED
			}
		case ADDING_TO_HUNK:
			hunk.addLine(reader.line)
			last := addAllEditsOnCurLine(hunk, reader, editIter)
			if reader.nextLineIsAffectedBy(last) {
				curState = ADDING_TO_HUNK
			} else {
				if reader.editAddsToStart(last) {
					trailingCtxLines = 1
				} else {
					trailingCtxLines = 0
				}
				curState = EDIT_ADDED_TO_HUNK
			}
		case EDIT_ADDED_TO_HUNK:
			hunk.addLine(reader.line)
			if reader.currentLineIsAffectedBy(editIter.edit()) {
				last := addAllEditsOnCurLine(hunk, reader, editIter)
				if reader.nextLineIsAffectedBy(last) {
					curState = ADDING_TO_HUNK
				} else {
					if reader.editAddsToStart(last) {
						trailingCtxLines = 1
					} else {
						trailingCtxLines = 0
					}
					curState = EDIT_ADDED_TO_HUNK
				}
			} else {
				trailingCtxLines++
				if trailingCtxLines < 2*num_ctx_lines {
					curState = EDIT_ADDED_TO_HUNK
				} else {
					result.add(hunk)
					hunk = nil
					curState = HUNK_NOT_STARTED
				}
			}
		}
	}
	if curState == ADDING_TO_HUNK || curState == EDIT_ADDED_TO_HUNK {
		if reader.line != "" {
			hunk.addLine(reader.line)
		}
		if curState == ADDING_TO_HUNK {
			addAllEditsOnCurLine(hunk, reader, editIter)
		}
		result.add(hunk)
	}
	if err == io.EOF {
		err = nil
	}
	return
}

func addAllEditsOnCurLine(hunk *hunk, reader *lineRdr, editIter *editIter) *edit {
	var lastEdit *edit = editIter.edit()
	for reader.currentLineIsAffectedBy(editIter.edit()) {
		if reader.nextLineIsAffectedBy(editIter.edit()) {
			return lastEdit
		} else {
			lastEdit = editIter.edit()
			hunk.addEdit(editIter.edit())
			editIter.moveToNextEdit()
		}
	}
	return lastEdit
}
