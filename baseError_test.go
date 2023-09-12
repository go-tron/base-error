package baseError

import "testing"

func TestNewBaseError(t *testing.T) {
	err := New("23", "33")
	t.Log(err)
}
