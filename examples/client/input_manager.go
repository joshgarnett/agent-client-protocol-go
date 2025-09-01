package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	// requestBufferSize is the buffer size for input requests channel.
	requestBufferSize = 10
)

// InputRequest represents a request for user input.
type InputRequest struct {
	Prompt   string
	Options  []string // If provided, user must choose from these options
	Response chan<- string
}

// InputManager handles all stdin input in a single goroutine to avoid conflicts.
type InputManager struct {
	requests chan InputRequest
	shutdown chan struct{}
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewInputManager creates a new input manager.
func NewInputManager() *InputManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &InputManager{
		requests: make(chan InputRequest, requestBufferSize),
		shutdown: make(chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins processing input requests.
func (im *InputManager) Start() {
	im.wg.Add(1)
	go im.inputLoop()
}

// Stop shuts down the input manager.
func (im *InputManager) Stop() {
	im.cancel()
	close(im.shutdown)
	im.wg.Wait()
}

// RequestInput requests input from the user with an optional prompt.
func (im *InputManager) RequestInput(prompt string) (string, error) {
	return im.RequestInputWithOptions(prompt, nil)
}

// RequestInputWithOptions requests input with specific options.
func (im *InputManager) RequestInputWithOptions(prompt string, options []string) (string, error) {
	response := make(chan string, 1)

	select {
	case im.requests <- InputRequest{
		Prompt:   prompt,
		Options:  options,
		Response: response,
	}:
		// Request queued successfully
	case <-im.ctx.Done():
		return "", im.ctx.Err()
	}

	// Wait for response
	select {
	case result := <-response:
		return result, nil
	case <-im.ctx.Done():
		return "", im.ctx.Err()
	}
}

// inputLoop processes input requests in a single goroutine.
func (im *InputManager) inputLoop() {
	defer im.wg.Done()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		select {
		case req := <-im.requests:
			result := im.handleInputRequest(scanner, req)
			select {
			case req.Response <- result:
				// Response sent
			case <-im.ctx.Done():
				return
			}

		case <-im.shutdown:
			return

		case <-im.ctx.Done():
			return
		}
	}
}

// handleOptionSelection handles selection from multiple options.
func (im *InputManager) handleOptionSelection(scanner *bufio.Scanner, options []string) string {
	fmt.Println()
	for i, option := range options {
		fmt.Printf("  %d. %s\n", i+1, option)
	}
	fmt.Print("Choose an option (1-" + strconv.Itoa(len(options)) + "): ")

	for {
		if !scanner.Scan() {
			// EOF or error - return first option as default
			if len(options) > 0 {
				fmt.Printf("Defaulting to: %s\n", options[0])
				return "1"
			}
			return ""
		}

		// Check for scan error
		if scanner.Err() != nil {
			if len(options) > 0 {
				fmt.Printf("Input error, defaulting to: %s\n", options[0])
				return "1"
			}
			return ""
		}

		choice := strings.TrimSpace(scanner.Text())
		if choice == "" {
			// For option selection, don't accept empty input - ask again
			fmt.Print("Choose an option (1-" + strconv.Itoa(len(options)) + "): ")
			continue
		}

		choiceNum, err := strconv.Atoi(choice)
		if err != nil || choiceNum < 1 || choiceNum > len(options) {
			fmt.Println("Invalid choice. Please try again.")
			fmt.Print("Choose an option (1-" + strconv.Itoa(len(options)) + "): ")
			continue
		}

		return choice
	}
}

// handleInputRequest processes a single input request.
func (im *InputManager) handleInputRequest(scanner *bufio.Scanner, req InputRequest) string {
	// Display prompt
	if req.Prompt != "" {
		fmt.Print(req.Prompt)
	}

	// If options are provided, handle option selection
	if len(req.Options) > 0 {
		return im.handleOptionSelection(scanner, req.Options)
	}

	// Simple text input
	if !scanner.Scan() {
		// Check for scan error or EOF
		if scanner.Err() != nil {
			return ""
		}
		// EOF - return empty string
		return ""
	}

	text := strings.TrimSpace(scanner.Text())
	// For regular prompts, don't accept empty input (except from EOF)
	if text == "" {
		// This was likely a pipe with single input followed by EOF
		// Return empty to signal completion
		return ""
	}
	return text
}
