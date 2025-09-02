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

// ============================================================================
// Input Manager Configuration
// ============================================================================

const (
	// requestBufferSize is the input request channel buffer size.
	requestBufferSize = 10
)

// ============================================================================
// Input Management Types and Implementation
// ============================================================================

// InputRequest represents a user input request.
type InputRequest struct {
	Prompt   string
	Options  []string // Optional predefined choices
	Response chan<- string
}

// InputManager serializes stdin input to avoid conflicts.
type InputManager struct {
	requests chan InputRequest
	shutdown chan struct{}
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewInputManager creates an input manager.
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

// RequestInput requests user input with optional prompt.
func (im *InputManager) RequestInput(prompt string) (string, error) {
	return im.RequestInputWithOptions(prompt, nil)
}

// RequestInputWithOptions requests input with predefined choices.
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

// inputLoop processes input requests sequentially.
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

// handleOptionSelection prompts user to choose from options.
func (im *InputManager) handleOptionSelection(scanner *bufio.Scanner, options []string) string {
	fmt.Println()
	for i, option := range options {
		fmt.Printf("  %d. %s\n", i+1, option)
	}
	fmt.Print("Choose an option (1-" + strconv.Itoa(len(options)) + "): ")

	for {
		if !scanner.Scan() {
			// EOF or error - use first option as default
			if len(options) > 0 {
				fmt.Printf("Defaulting to: %s\n", options[0])
				return "1"
			}
			return ""
		}

		// Handle scan error
		if scanner.Err() != nil {
			if len(options) > 0 {
				fmt.Printf("Input error, defaulting to: %s\n", options[0])
				return "1"
			}
			return ""
		}

		choice := strings.TrimSpace(scanner.Text())
		if choice == "" {
			// Require valid selection
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

// handleInputRequest processes an input request.
func (im *InputManager) handleInputRequest(scanner *bufio.Scanner, req InputRequest) string {
	// Show prompt
	if req.Prompt != "" {
		fmt.Print(req.Prompt)
	}

	// Handle option selection if provided
	if len(req.Options) > 0 {
		return im.handleOptionSelection(scanner, req.Options)
	}

	// Handle text input
	if !scanner.Scan() {
		// Handle scan error or EOF
		if scanner.Err() != nil {
			return ""
		}
		// EOF - return empty
		return ""
	}

	text := strings.TrimSpace(scanner.Text())
	// Handle empty input
	if text == "" {
		// Pipe input completed
		return ""
	}
	return text
}
