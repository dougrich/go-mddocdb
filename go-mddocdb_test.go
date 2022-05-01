package mddcdb_test

import (
	"testing"

	"github.com/dougrich/go-mddocdb"
)

func TestPlaceholder(t *testing.T) {
	if mddcdb.Placeholder() != 2 {
		t.Fatal("Expected a placeholder value of 2")
	}
}
