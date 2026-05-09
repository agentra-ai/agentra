package hooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncate(t *testing.T) {
	result := truncate("short", 10)
	assert.Equal(t, "short", result)

	result = truncate("this is a long string", 10)
	assert.Equal(t, "this is a ...", result)
	assert.Len(t, result, 13) // "this is a " + "..."

	result = truncate("", 10)
	assert.Equal(t, "", result)
}

func TestTaskCompletionHook_Struct(t *testing.T) {
	// Verify the struct can be created
	hook := NewTaskCompletionHook(nil)
	assert.NotNil(t, hook)
}

func TestTaskStartHook_Struct(t *testing.T) {
	// Verify the struct can be created with default limit
	hook := NewTaskStartHook(nil, 0)
	assert.NotNil(t, hook)
	assert.Equal(t, 5, hook.injectLimit)

	// Verify custom limit
	hook = NewTaskStartHook(nil, 10)
	assert.Equal(t, 10, hook.injectLimit)
}