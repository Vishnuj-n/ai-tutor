package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type FunctionInfo struct {
	Name     string
	File     string
	Receiver string
	Params   []string
	Body     string
}

type DuplicateAnalysis struct {
	Functions    map[string][]FunctionInfo
	Structs      map[string][]string
	Interfaces   map[string][]string
	Imports      map[string][]string
	Similarities []Similarity
}

type Similarity struct {
	Type        string
	Name        string
	Files       []string
	Similarity  float64
	Description string
}

func main() {
	dbPath := "../internal/db"
	analysis := analyzeDuplicates(dbPath)
	fmt.Println("=== DB FOLDER DUPLICATE ANALYSIS ===")
	fmt.Println()

	printDuplicateFunctions(analysis)
	printDuplicateStructs(analysis)
	printRedundantImports(analysis)
	printSimilarities(analysis)
	printRecommendations(analysis)
}

func analyzeDuplicates(dir string) *DuplicateAnalysis {
	analysis := &DuplicateAnalysis{
		Functions:  make(map[string][]FunctionInfo),
		Structs:    make(map[string][]string),
		Interfaces: make(map[string][]string),
		Imports:    make(map[string][]string),
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		analyzeFile(path, analysis)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk error for %s: %v\n", dir, err)
	}

	findSimilarities(analysis)
	return analysis
}

func analyzeFile(filename string, analysis *DuplicateAnalysis) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return
	}

	// Extract imports
	for _, imp := range node.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		analysis.Imports[path] = append(analysis.Imports[path], filename)
	}

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Name.IsExported() {
				funcInfo := FunctionInfo{
					Name: x.Name.Name,
					File: filename,
				}

				if x.Recv != nil && len(x.Recv.List) > 0 {
					if starExpr, ok := x.Recv.List[0].Type.(*ast.StarExpr); ok {
						if ident, ok := starExpr.X.(*ast.Ident); ok {
							funcInfo.Receiver = ident.Name
						}
					}
				}

				key := funcInfo.Name
				if funcInfo.Receiver != "" {
					key = funcInfo.Receiver + "." + funcInfo.Name
				}

				analysis.Functions[key] = append(analysis.Functions[key], funcInfo)
			}

		case *ast.TypeSpec:
			if x.Name.IsExported() {
				switch x.Type.(type) {
				case *ast.StructType:
					analysis.Structs[x.Name.Name] = append(analysis.Structs[x.Name.Name], filename)
				case *ast.InterfaceType:
					analysis.Interfaces[x.Name.Name] = append(analysis.Interfaces[x.Name.Name], filename)
				}
			}
		}
		return true
	})
}

func findSimilarities(analysis *DuplicateAnalysis) {
	// Check for similar function patterns
	patterns := map[string][]string{
		"CRUD Operations":    {"Create", "Get", "Update", "Delete", "List"},
		"Repository Pattern": {"ByID", "ByName", "All", "Save", "Remove"},
		"Test Helpers":       {"Setup", "Teardown", "Mock", "Stub"},
	}

	for pattern, keywords := range patterns {
		files := make(map[string]bool)
		for funcName := range analysis.Functions {
			for _, keyword := range keywords {
				if strings.Contains(funcName, keyword) {
					for _, funcInfo := range analysis.Functions[funcName] {
						files[funcInfo.File] = true
					}
				}
			}
		}

		if len(files) > 1 {
			fileList := make([]string, 0, len(files))
			for file := range files {
				fileList = append(fileList, filepath.Base(file))
			}

			analysis.Similarities = append(analysis.Similarities, Similarity{
				Type:        "Pattern",
				Name:        pattern,
				Files:       fileList,
				Similarity:  0.8,
				Description: fmt.Sprintf("Multiple files implement %s pattern", pattern),
			})
		}
	}
}

func printDuplicateFunctions(analysis *DuplicateAnalysis) {
	fmt.Println("🔍 DUPLICATE FUNCTIONS:")
	found := false

	for funcName, funcs := range analysis.Functions {
		if len(funcs) > 1 {
			found = true
			fmt.Printf("  ⚠️  %s appears in %d files:\n", funcName, len(funcs))
			for _, f := range funcs {
				fmt.Printf("     - %s\n", filepath.Base(f.File))
			}
			fmt.Println()
		}
	}

	if !found {
		fmt.Println("  ✅ No duplicate function names found")
		fmt.Println()
	}
}

func printDuplicateStructs(analysis *DuplicateAnalysis) {
	fmt.Println("📦 DUPLICATE STRUCTS/INTERFACES:")
	found := false

	for structName, files := range analysis.Structs {
		if len(files) > 1 {
			found = true
			fmt.Printf("  ⚠️  Struct '%s' defined in:\n", structName)
			for _, file := range files {
				fmt.Printf("     - %s\n", filepath.Base(file))
			}
		}
	}

	for intName, files := range analysis.Interfaces {
		if len(files) > 1 {
			found = true
			fmt.Printf("  ⚠️  Interface '%s' defined in:\n", intName)
			for _, file := range files {
				fmt.Printf("     - %s\n", filepath.Base(file))
			}
		}
	}

	if !found {
		fmt.Println("  ✅ No duplicate struct/interface definitions")
		fmt.Println()
	}
}

func printRedundantImports(analysis *DuplicateAnalysis) {
	fmt.Println("📥 COMMON IMPORTS (potential for shared utilities):")

	commonImports := make(map[string]int)
	for imp, files := range analysis.Imports {
		if len(files) >= 3 && !strings.Contains(imp, "std") {
			commonImports[imp] = len(files)
		}
	}

	if len(commonImports) > 0 {
		for imp, count := range commonImports {
			fmt.Printf("  📦 %s used in %d files\n", imp, count)
		}
	} else {
		fmt.Println("  ✅ No heavily shared external imports")
	}
	fmt.Println()
}

func printSimilarities(analysis *DuplicateAnalysis) {
	fmt.Println("🔄 PATTERN SIMILARITIES:")

	if len(analysis.Similarities) > 0 {
		for _, sim := range analysis.Similarities {
			fmt.Printf("  🎯 %s (%.0f%% similar)\n", sim.Name, sim.Similarity*100)
			fmt.Printf("     %s\n", sim.Description)
			fmt.Printf("     Files: %s\n\n", strings.Join(sim.Files, ", "))
		}
	} else {
		fmt.Println("  ✅ No obvious pattern duplications detected")
		fmt.Println()
	}
}

func printRecommendations(analysis *DuplicateAnalysis) {
	fmt.Println("💡 RECOMMENDATIONS:")

	repoFiles := 0
	testFiles := 0

	for funcName := range analysis.Functions {
		for _, funcInfo := range analysis.Functions[funcName] {
			if strings.Contains(funcInfo.File, "_repo.go") {
				repoFiles++
			}
			if strings.Contains(funcInfo.File, "_test.go") {
				testFiles++
			}
		}
	}

	fmt.Printf("  📊 Found %d repository files\n", countUniqueFiles(analysis.Functions, "_repo.go"))
	fmt.Printf("  🧪 Found %d test files\n", countUniqueFiles(analysis.Functions, "_test.go"))

	fmt.Println("\n  🎯 Potential optimizations:")
	fmt.Println("     - Consider shared base repository interface")
	fmt.Println("     - Extract common CRUD patterns to shared utilities")
	fmt.Println("     - Consolidate test helpers in testhelper.go")
	fmt.Println("     - Review if separate repos can be merged based on domain")
}

func countUniqueFiles(functions map[string][]FunctionInfo, suffix string) int {
	files := make(map[string]bool)
	for _, funcs := range functions {
		for _, f := range funcs {
			if strings.Contains(f.File, suffix) {
				files[f.File] = true
			}
		}
	}
	return len(files)
}
