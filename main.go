package main

import (
	"bytes"
	"fmt"
	"go/build"
	"go/types"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/interp"
	"golang.org/x/tools/go/ssa/ssautil"
)

func main() {
	l := log.New(os.Stdout, "ssa-", log.LstdFlags)
	run(l, "cmd/simple/simple.go")
}

func run(t *log.Logger, input string) bool {
	t.Printf("Input: %s\n", input)

	start := time.Now()

	ctx := build.Default // copy
	gopath := os.Getenv("GOPATH")
	ctx.GOROOT = gopath + "/src/golang.org/x/tools/go/ssa/interp/testdata"
	ctx.GOOS = "linux"
	ctx.GOARCH = "amd64"

	conf := loader.Config{Build: &ctx}
	if _, err := conf.FromArgs([]string{input}, true); err != nil {
		t.Printf("FromArgs(%s) failed: %s", input, err)
		return false
	}

	conf.Import("runtime")

	// Print a helpful hint if we don't make it to the end.
	var hint string
	defer func() {
		if hint != "" {
			fmt.Println("FAIL")
			fmt.Println(hint)
		} else {
			fmt.Println("PASS")
		}

		interp.CapturedOutput = nil
	}()

	hint = fmt.Sprintf("To dump SSA representation, run:\n%% go build golang.org/x/tools/cmd/ssadump && ./ssadump -test -build=CFP %s\n", input)

	iprog, err := conf.Load()
	if err != nil {
		t.Printf("conf.Load(%s) failed: %s", input, err)
		return false
	}

	prog := ssautil.CreateProgram(iprog, ssa.SanityCheckFunctions)
	prog.Build()

	mainPkg := prog.Package(iprog.Created[0].Pkg)
	if mainPkg == nil {
		t.Fatalf("not a main package: %s", input)
	}

	interp.CapturedOutput = new(bytes.Buffer)

	hint = fmt.Sprintf("To trace execution, run:\n%% go build golang.org/x/tools/cmd/ssadump && ./ssadump -build=C -test -run --interp=T %s\n", input)
	exitCode := interp.Interpret(mainPkg, 0, &types.StdSizes{WordSize: 8, MaxAlign: 8}, input, []string{})
	if exitCode != 0 {
		t.Fatalf("interpreting %s: exit code was %d", input, exitCode)
	}
	// $GOROOT/test tests use this convention:
	if strings.Contains(interp.CapturedOutput.String(), "BUG") {
		t.Fatalf("interpreting %s: exited zero but output contained 'BUG'", input)
	}

	hint = "" // call off the hounds

	if false {
		t.Printf(input, time.Since(start)) // test profiling
	}

	return true
}
