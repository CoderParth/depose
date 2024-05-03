package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMain(t *testing.T) {
	// Build the Go application
	cmd := exec.Command("go", "build")
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to build application: %v", err)
	}

	// Move the built file to the test directory
	err = os.Rename("purge", "test/purge")
	if err != nil {
		t.Fatalf("Failed to move built file: %v", err)
	}

	// Get the working directory,
	// Combine the directory, and
	// Run the built file
	dir, _ := os.Getwd()
	purgePath := filepath.Join(dir, "test", "purge")
	cmd = exec.Command(purgePath)
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Dir(purgePath)
	err = cmd.Run()
	if err != nil {
		t.Fatalf("Failed to run built file: %v", err)
	}

	// Read the package.json file and expected.json file
	// And Compare them
	packageJSON, err := os.ReadFile("test/package.json")
	if err != nil {
		t.Fatalf("Failed to read package.json: %v", err)
	}

	expectedJSON, err := os.ReadFile("test/expected.json")
	if err != nil {
		t.Fatalf("Failed to read expected.json: %v", err)
	}

	// Unmarshal the JSON data
	var packageData map[string]interface{}
	var expectedData map[string]interface{}

	err = json.Unmarshal(packageJSON, &packageData)
	if err != nil {
		t.Fatalf("Failed to unmarshal package.json: %v", err)
	}

	err = json.Unmarshal(expectedJSON, &expectedData)
	if err != nil {
		t.Fatalf("Failed to unmarshal expected.json: %v", err)
	}

	// Compare the data
	if !reflect.DeepEqual(packageData, expectedData) {
		t.Fatalf("package.json does not match expected.json")
	}
}
