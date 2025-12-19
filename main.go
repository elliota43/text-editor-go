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
	ARROW_UP = iota + 1000
	ARROW_DOWN
	ARROW_LEFT
	ARROW_RIGHT
)

type Editor struct {
	Cx, Cy        int
	Width, Height int
}

func NewEditor(width, height int) *Editor {
	return &Editor{
		Cx:     0,
		Cy:     0,
		Width:  width,
		Height: height,
	}
}

func (e *Editor) RefreshScreen() {
	var b strings.Builder

	b.WriteString("\x1b[?25l") // Hide cursor (avoids flickering effect)
	b.WriteString("\x1b[H")    // Move to 1,1

	e.drawRows(&b)
	e.drawStatus(&b)

	// move cursor to where the user is
	b.WriteString(fmt.Sprintf("\x1b[%d;%dH", e.Cy+1, e.Cx+1))

	b.WriteString("\x1b[?25h") // show cursor again
	os.Stdout.WriteString(b.String())
}

func (e *Editor) drawRows(b *strings.Builder) {
	for y := 0; y < e.Height-1; y++ {
		b.WriteString("\x1b[K") // Clear line
		b.WriteString("~")
		if y < e.Height-2 {
			b.WriteString("\r\n")
		}
	}
}

func (e *Editor) drawStatus(b *strings.Builder) {
	// Move to the last line
	b.WriteString(fmt.Sprintf("\x1b[%d;1H", e.Height))

	// Clear the line
	b.WriteString("\x1b[K")

	// Invert colors (styling)
	b.WriteString(fmt.Sprintf("\x1b[7m Cursor: %d, %d \x1b[m", e.Cx, e.Cy))
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
		case 27: // The ESC byte
			// Check if there is more data immediately available
			if reader.Buffered() == 0 {
				// just the scape key
				fmt.Printf("\r\nPressed: ESC")
				return
			}
			// If it's aa sequence, peek at the next byte
			next, _ := reader.Peek(1)
			if next[0] == '[' {
				reader.ReadByte()

				// Read the finalizer byte (A, B, C, or D)
				final, _ := reader.ReadByte()
				switch final {
				case 'A':
					//fmt.Print("\r\nPressed: UP")
					if editor.Cy > 0 {
						editor.Cy--
					}
				case 'B':
					//fmt.Print("\r\nPressed: DOWN")
					if editor.Cy < editor.Height-2 {
						editor.Cy++
					}
				case 'C':

					if editor.Cx < editor.Width-1 {
						editor.Cx++
					}
					//fmt.Print("\r\nPressed: RIGHT")
				case 'D':
					if editor.Cx > 0 {
						editor.Cx--
					}
					//fmt.Print("\r\nPressed: LEFT")
				}
			}

		case 'q':
			os.Stdout.WriteString("\x1b[2J") // Clear screen
			os.Stdout.WriteString("\x1b[H")  // Reset cursor
			return                           //Exit loop

		default:
			fmt.Printf("\r\nRead: %d (char: %c)", char, char)
		}
	}
}
