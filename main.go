package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Dependency struct uses map to store name of dependencies and
// a mutex for concurrent access.
//
// Dependencies with falsy values are deleted at the end.
type Dependency struct {
	mp map[string]bool
	mu sync.Mutex
}

// Package struct represents the keys of the package.json file,
// which are accessed when readPackages() is called.
//
// Its instance is used to Unmarshal the JSON data from the package.json file.
type Package struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

var (
	d Dependency
	// filesToExclude represents a map of file names/directories
	// which are supposed to be skipped during the process of scanning
	// the whole directory.
	filesToExclude = map[string]int{
		"node_modules":      0,
		".gitignore":        0,
		".git":              0,
		".env":              0,
		"package.json":      0,
		"package-lock.json": 0,
		"README.md":         0,
		"main.go":           0,
		"depose":            0,
	}
	// wg is a collection of go routines, which is also used
	// to wait for all the goroutines to finish their processes.
	wg sync.WaitGroup
)

// readPackages reads the package.json file,
// unmarshals the data to the instance of Package called "pkg",
// and populates the map of globally declared instance of Dependency
// struct called "d".
//
// The dependencies and dev dependencies found in package.json file
// are stored initially in the map with falsy values. Later, in the Program
// when those dependencies are found in other files, these values are updated
// to true.
//
// The dependencies seen in "scripts" section of the package.json file
// is initialzed as true because though the dependency might not be required
// elsewhere in other files, it might still have other external duties in the project.
// These type of external dependencies are not deleted.
func readPackages() {
	jsonFile, err := os.Open("package.json")
	if err != nil {
		log.Fatal(err)
	}

	defer jsonFile.Close()

	fmt.Println("Reading Package.json")

	byteValue, _ := io.ReadAll(jsonFile)
	var pkg Package
	json.Unmarshal(byteValue, &pkg)

	for dependency := range pkg.Dependencies {
		d.mp[dependency] = false
	}

	for dependency := range pkg.DevDependencies {
		d.mp[dependency] = false
	}

	// Mark the dependencies used in the scripts section as true i.e. do not remove them.
	for _, script := range pkg.Scripts {
		for dependency := range d.mp {
			if strings.Contains(script, dependency) {
				d.mp[dependency] = true
			}
		}
	}
}

// scnaDir is the function called by filePath.Walk to visit each
// file or directory.
//
// The files and dirs included in the "filesToExclude" map are skipped.
// The other files are read, and packages are extracted from them
// concurrently.
func scanDir(path string, info fs.FileInfo, e error) error {
	if _, ok := filesToExclude[path]; ok {
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}

	if !info.IsDir() {
		wg.Add(1)
		go readFileAndExtractPackages(path)
	}
	return nil
}

// readFileAndExtractPackages is a concurrent process, which
// opens up the file provided as the argument to the function,
// then the file is read line by line, and is passed to scanLineAndExtractPkgs
func readFileAndExtractPackages(file string) {
	defer wg.Done()

	readFile, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}

	defer readFile.Close()

	fmt.Printf("Reading file: %s\n", file)
	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		currLine := fileScanner.Text()
		scanLineAndExtractPkgs(currLine)
	}
}

// scanLineAndExtractPkgs takes the the line as an argument,
// and checks if "require" keyword or "import" keyword is present in the line,
// and calls other functions to handle the case based on it.
func scanLineAndExtractPkgs(currLine string) {
	// for case where "require" keyword is used.
	hasRequireKeyword := strings.Contains(currLine, "require")
	if hasRequireKeyword {
		handleRequireCase(currLine)
	}

	// for case where "import" keyword is used.
	hasImportKeyword := strings.Contains(currLine, "import")
	if hasImportKeyword {
		handleImportCase(currLine)
	}
}

func handleRequireCase(currLine string) {
	pkgs := strings.Split(currLine, `require("`)
	for i, v := range pkgs {
		// First index contains empty string, so skip.
		if i == 0 {
			continue
		}
		if !strings.HasPrefix(v, ".") { // "." is associated with file imports, so it's skipped.
			moduleName := strings.TrimSuffix(v, `");`)
			fmt.Printf("Found a packge: %v\n", moduleName)
			markModuleAsFound(moduleName)
		}
	}
}

func handleImportCase(currLine string) {
	// Regular expression to match module names in import statements
	re := regexp.MustCompile(`from\s*["']([^"']+)["']|import\s*["']([^"']+)["']`)
	matches := re.FindAllStringSubmatch(currLine, -1)

	for _, match := range matches {
		// The first submatch is the module name like "import ... from 'module-name'",
		// and the second submatch is the module name like "import 'module-name'".
		// One of them will be empty, and one will contain the module name.
		moduleName := match[1]
		if moduleName == "" {
			moduleName = match[2]
		}

		fmt.Printf("Found a package: %v\n", moduleName)
		markModuleAsFound(moduleName)
	}
}

// markModuleAsFound locks the mutex of globally declared instance of
// dependency called "d", updates the module/dependency as true, and
// then unlocks it again.
func markModuleAsFound(moduleName string) {
	d.mu.Lock()

	if _, ok := d.mp[moduleName]; ok {
		d.mp[moduleName] = true
	}

	d.mu.Unlock()
}

// Create a list of dependencies to remove, based on falsy values of d.mp
func createDepsToRemoveList() []string {
	var depsToRemove []string
	for k, v := range d.mp {
		if !v {
			depsToRemove = append(depsToRemove, k)
		}
		fmt.Printf("Removing Package: %v\n", k)
	}
	return depsToRemove
}

// deleteDepsFromPackageJSON opens up the package.json file,
// creates a new file called "newpackage.json", copies the
// contents of the package.json to newpackage.json. However,
// lines containing the dependency from "depsToRemove" are not copied
// to the newpackage.json file.
//
// The removeTrailingCommas function is called inside deleteDepsFromPackageJSON
// to fix the syntax of the newpackage.json file.
//
// The current package.json file is renamed to oldpackage.json for further
// reviews and for the users to make final changes, before deleting that file.
//
// Similarly, the newpackage.json is renamed as package.json file.
func deleteDepsFromPackageJSON(depsToRemove []string) {
	jsonFile, err := os.Open("package.json")
	if err != nil {
		log.Fatal(err)
	}

	createNewPackageJsonFile(depsToRemove, jsonFile)
	removeTrailingCommas()

	os.Rename("package.json", "oldpackage.json")
	os.Rename("newPackage.json", "package.json")
}

// createNewPackageJsonFile creates a new
// file called "newpackage.json", copies the
// contents of the package.json to newpackage.json. However,
// lines containing the dependency from "depsToRemove" are not copied
// to the newpackage.json file.
func createNewPackageJsonFile(depsToRemove []string, jsonFile *os.File) {
	newFile, err := os.Create("newPackage.json")
	if err != nil {
		log.Fatal(err)
	}
	// Create a writer for the new file
	writer := bufio.NewWriter(newFile)
	scanner := bufio.NewScanner(jsonFile)
	for scanner.Scan() { // scan line by line
		line := scanner.Text()
		// Check if the line contains a dependency to remove
		shouldWrite := true
		for _, dep := range depsToRemove {
			if strings.Contains(line, dep) {
				shouldWrite = false
				break // skip the line
			}
		}
		if shouldWrite {
			writer.WriteString(line + "\n")
		}
	}
	writer.Flush() // Flush to make sure all data is written to newFile
	newFile.Close()
	jsonFile.Close()
}

// The removeTrailingCommas function is called from inside deleteDepsFromPackageJSON.
// It fixes the syntax of newpackage.json file.
//
// With this function, the trailing commas which remain after the deletion of the dependency
// are removed to fix the syntax.
//
// Example:
//
//	"devDependencies": {
//	  "jest": "^29.7.0", <- In cases like this, this comma here is removed
//	}
func removeTrailingCommas() {
	data, err := os.ReadFile("newPackage.json")
	if err != nil {
		log.Fatal(err)
	}
	// Convert the byte slice to a string
	json := string(data)
	// Use a regular expression to remove trailing commas before closed curlybraces "}"
	re := regexp.MustCompile(`,\s*}`)
	json = re.ReplaceAllString(json, "}")

	// Write the byte slice back to the newPackage.json file
	data = []byte(json)
	err = os.WriteFile("newPackage.json", data, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	// initialization of an empty map to store dependencies
	d.mp = make(map[string]bool)

	readPackages()
	// Walk the directory, and scan each directory/file.
	if err := filepath.Walk(".", scanDir); err != nil {
		fmt.Printf("Error scanning the directory %v:\n", err)
	}

	wg.Wait() // wait for all goroutines to finish
	fmt.Println("Finished walking the directory")

	depsToRemove := createDepsToRemoveList()
	deleteDepsFromPackageJSON(depsToRemove)

	fmt.Println("Program Complete....")
	fmt.Println("Package.json has been changed.")
	fmt.Println("Refer to oldpackage.json for the old original file.")
}
