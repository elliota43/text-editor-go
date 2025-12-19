package main

import (
	"bufio"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"fmt"
	"golang.org/x/term"
)

const (
	KEY_ESC       = '\x1b'
	KEY_ENTER     = '\r'
	KEY_BACKSPACE = 127
	KeyTab        = '\t'
)

const (
	ARROW_UP = iota + 1000
	ARROW_DOWN
	ARROW_LEFT
	ARROW_RIGHT
)

type Editor struct {
	Cx, Cy        int
	Width, Height int
	Rows          []string
	RowOffset     int
	ColOffset     int
	Filename      string
}

func ctrlKey(char rune) rune {
	return char & 0x1f
}

func NewEditor(width, height int) *Editor {
	return &Editor{
		Cx:     0,
		Cy:     0,
		Width:  width,
		Height: height,
		Rows:   []string{""},
	}
}

func (e *Editor) Scroll() {
	// if cursor is above visible window, scroll up
	if e.Cy < e.RowOffset {
		e.RowOffset = e.Cy
	}

	// if cursor is below visible window, scroll down
	// (Height-1 bc last row is status bar)
	if e.Cy >= e.RowOffset+(e.Height-1) {
		e.RowOffset = e.Cy - (e.Height - 2)
	}

	// Horizontal Scroll Left
	if e.Cx < e.ColOffset {
		e.ColOffset = e.Cx
	}

	// Horiz. Scroll Right
	if e.Cx >= e.ColOffset+e.Width {
		e.ColOffset = e.Cx - e.Width + 1
	}

}

func (e *Editor) RefreshScreen() {
	e.Scroll()
	var b strings.Builder

	b.WriteString("\x1b[?25l") // Hide cursor (avoids flickering effect)
	b.WriteString("\x1b[H")    // Move to 1,1

	e.DrawRows(&b)
	e.DrawStatus(&b)

	// move cursor to where the user is
	b.WriteString(fmt.Sprintf("\x1b[%d;%dH", (e.Cy-e.RowOffset)+1, (e.Cx-e.ColOffset)+1))

	b.WriteString("\x1b[?25h") // show cursor again
	os.Stdout.WriteString(b.String())
}

func (e *Editor) DrawRows(b *strings.Builder) {
	for y := 0; y < e.Height-1; y++ {

		fileRowIndex := y + e.RowOffset
		b.WriteString("\x1b[K") // Clear line

		if fileRowIndex < len(e.Rows) {
			// draw the line from the buffer
			line := e.Rows[fileRowIndex]

			// skip characters to the left of ColOffset
			if len(line) > e.ColOffset {
				line = line[e.ColOffset:]
			} else {
				line = "" // entire line is scrolled off to the left
			}

			// trim characters beyond screen width
			if len(line) > e.Width {
				line = line[:e.Width]
			}
			b.WriteString(line)
		} else if len(e.Rows) == 1 && e.Rows[0] == "" && y == e.Height/3 {
			welcome := "GoEdit -- version 0.1"
			padding := (e.Width - len(welcome)) / 2
			b.WriteString(strings.Repeat(" ", padding) + welcome)
		} else {
			b.WriteString("~")
		}

		if y < e.Height-2 {
			b.WriteString("\r\n")
		}

	}
}

func (e *Editor) DrawStatus(b *strings.Builder) {
	// Move to the last line
	b.WriteString(fmt.Sprintf("\x1b[%d;1H", e.Height))

	// Clear the line
	b.WriteString("\x1b[K")

	// Invert colors (styling)
	b.WriteString(fmt.Sprintf("\x1b[7m Cursor: %d, %d \x1b[m", e.Cx, e.Cy))
}

func (e *Editor) snapCursorToText() {
	rowLen := 0
	if e.Cy < len(e.Rows) {
		rowLen = len(e.Rows[e.Cy])
	}

	if e.Cx > rowLen {
		e.Cx = rowLen
	}
}

func (e *Editor) HandleEscapeSequence(r *bufio.Reader) {
	// If it's aa sequence, peek at the next byte
	next, _ := r.Peek(1)
	if next[0] == '[' {
		r.ReadByte()

		// Read the finalizer byte (A, B, C, or D)
		final, _ := r.ReadByte()
		switch final {
		case 'A':
			if e.Cy > 0 {
				e.Cy--
			}
		case 'B':
			if e.Cy < len(e.Rows)-1 {
				e.Cy++
			}
		case 'C':

			if e.Cx < e.Width-1 {
				e.Cx++
			}
		case 'D':
			if e.Cx > 0 {
				e.Cx--
			}
		}

		e.snapCursorToText()
	}
}

func (e *Editor) insertRowAtIndex(index int, text string) {
	e.Rows = append(e.Rows, "")

	// shift by 1 to make space for new row
	copy(e.Rows[index+1:], e.Rows[index:])

	e.Rows[index] = text
}
func (e *Editor) InsertNewLine() {
	if e.Cx == 0 {
		// At start of line: insert empty row above
		e.Rows = append(e.Rows[:e.Cy], append([]string{""}, e.Rows[e.Cy:]...)...)
	} else {
		// in the middle of line: split row and insert
		// characters to the right of the cursor on the new line
		row := e.Rows[e.Cy]
		e.Rows[e.Cy] = row[:e.Cx]
		newRow := row[e.Cx:]
		e.insertRowAtIndex(e.Cy+1, newRow)
	}

	e.Cy++
	e.Cx = 0
}

func (e *Editor) DeleteChar() {
	// check if cursor is at the beginning of a new/empty file
	if e.Cy == 0 && e.Cx == 0 {
		return
	}

	row := e.Rows[e.Cy]

	if e.Cx > 0 {
		// check in middle of the line (remove char to the left)
		e.Rows[e.Cy] = row[:e.Cx-1] + row[e.Cx:]
		e.Cx--
	} else {
		// at beginning of line (join w previous line)
		prevRow := e.Rows[e.Cy-1]
		e.Cx = len(prevRow)

		e.Rows[e.Cy-1] = prevRow + row

		e.Rows = append(e.Rows[:e.Cy], e.Rows[e.Cy+1:]...)
	}
}

func (e *Editor) InsertChar(char rune) {
	for len(e.Rows) <= e.Cy {
		e.Rows = append(e.Rows, "")
	}

	row := e.Rows[e.Cy]

	// don't allow Cx to exceed row length when inserting
	if e.Cx > len(row) {
		e.Cx = len(row)
	}

	// [everything before] + [new char] + [everything after]
	e.Rows[e.Cy] = row[:e.Cx] + string(char) + row[e.Cx:]
	e.Cx++
}

func (e *Editor) Open(filename string) error {
	// Read entire file into memory
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// split content by newline characters into Rows slice
	e.Rows = strings.Split(string(content), "\n")
	e.Filename = filename
	e.Cx = 0
	e.Cy = 0
	return nil
}

func (e *Editor) Save() error {
	if e.Filename == "" {
		// @TODO: save-as prompt
		e.Filename = "goedit.txt"
	}

	output := strings.Join(e.Rows, "\n")

	err := os.WriteFile(e.Filename, []byte(output), 0644)
	return err
}

func main() {
	fd := int(os.Stdin.Fd())

	width, height, err := term.GetSize(fd)

	if err != nil {
		panic(err)
	}

	if !term.IsTerminal(fd) {
		panic("not a terminal")
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		panic(err)
	}

	// restore terminal to its original state
	defer term.Restore(fd, oldState)

	// graceful exit on Ctrl+C (SIGINT) and other signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		fmt.Println("\r\nExiting program and restoring terminal...")
		term.Restore(fd, oldState)
		os.Exit(1)
	}()

	fmt.Println("Terminal is in raw mode.  Press keys to see their byte values. Press Ctrl+C to exit.")

	editor := NewEditor(width, height)

	// Check if user specified a filename

	if len(os.Args) > 1 {
		editor.Open(os.Args[1])
	}

	reader := bufio.NewReader(os.Stdin)

	// Infinite loop to read bytes one by one
	for {
		// Draw
		editor.RefreshScreen()

		// Input
		char, _, err := reader.ReadRune()

		if err != nil {
			break
		}

		// Process
		switch char {
		case KEY_ESC: // The ESC byte
			if reader.Buffered() == 0 {
				continue
			}

			editor.HandleEscapeSequence(reader)

		case KEY_ENTER:
			editor.InsertNewLine()

		case KEY_BACKSPACE:
			editor.DeleteChar()

		case ctrlKey('q'):
			os.Stdout.WriteString("\x1b[2J") // Clear screen
			os.Stdout.WriteString("\x1b[H")  // Reset cursor

			return // exit

		case ctrlKey('s'):
			editor.Save()

		default:
			editor.InsertChar(char)
		}
	}
}
