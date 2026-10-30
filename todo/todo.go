// Package todo provides functions to persist a simple todo list to disk
// as JSON.
//
// Homework — Task 1 (Lesson 6: File I/O, JSON and Testing):
// Implement SaveTodos and LoadTodos below so that all tests in
// todo_test.go pass, and reach at least 80% statement coverage for
// this package (checked automatically by CI — see the repository
// README for how to run it locally).
package todo

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Todo represents a single todo-list item.
type Todo struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

// SaveTodos writes the given todos to the file at path as JSON,
// creating the file if it does not exist and overwriting it if it does.
//
// TODO: implement this function.
//   - Marshal todos to JSON (json.Marshal or json.MarshalIndent).
//   - Write the result to path (os.WriteFile is the simplest option).
//   - Wrap any error with context using fmt.Errorf("...: %w", err).
func SaveTodos(path string, todos []Todo) error {
	data, err := json.MarshalIndent(todos, "", "  ")
	if err != nil {
		return fmt.Errorf("SaveTodos: encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("SaveTodos: write %s: %w", path, err)
	}
	return nil
}

// LoadTodos reads and parses the todo list stored at path.
//
// TODO: implement this function.
//   - Read the file at path (os.ReadFile).
//   - Unmarshal the JSON bytes into a []Todo.
//   - Wrap errors so callers can tell "file missing" apart from
//     "malformed JSON" if they inspect the error (e.g. via errors.Is
//     against os.ErrNotExist, or by checking for a *json.SyntaxError).
func LoadTodos(path string) ([]Todo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadTodos: read %s: %w", path, err)
	}

	var todos []Todo
	if err := json.Unmarshal(data, &todos); err != nil {
		return nil, fmt.Errorf("LoadTodos: parse %s: %w", path, err)
	}
	return todos, nil
}
