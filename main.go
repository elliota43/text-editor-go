package main

import (
	"bufio"
	"io"
	"os"
	"os/signal"
	"syscall"

	"fmt"
	"golang.org/x/term"
)

func main() {

	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		fmt.Println("Stdin is not a terminal")
		return
	}

	// Save the original terminal state
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		panic(err)
	}

	// Restore the terminal to its original state on exit
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

	reader := bufio.NewReader(os.Stdin)

	// Infinite loop to read bytes one by one
	for {
		// Read a single byte
		char, _, err := reader.ReadRune()

		if err != nil {
			break
		}

		switch char {
		case 27: // The ESC byte
			// Check if there is more data immediately available
			if reader.Buffered() > 0 {
				// just the scape key
				fmt.Printf("\r\nPressed: ESC")
				return
			}
		}

		// If it's aa sequence, peek at the next byte
		next, _ := reader.Peek(1)
		if next[0] == '[' {
			reader.ReadByte()

			// Read the finalizer byte (A, B, C, or D)
			final, _ := reader.ReadByte()
			switch final {
			case 'A':
				fmt.Print("\r\nPressed: UP")
			case 'B':
				fmt.Print("\r\nPressed: DOWN")
			case 'C':
				fmt.Print("\r\nPressed: RIGHT")
			case 'D':
				fmt.Print("\r\nPressed: LEFT")
			}
		}

	}
}
