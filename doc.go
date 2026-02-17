// Package agentfile provides parsing, validation, and type definitions
// for the Agentfile v1 specification.
//
// The Agentfile is a declarative YAML format for packaging AI agents.
// This package is the canonical Go implementation of the specification,
// suitable for use by both the crate CLI and the crated runtime.
//
// Usage:
//
//	import "github.com/agentcrate/agentfile"
//
//	data, _ := os.ReadFile("Agentfile")
//	result, err := agentfile.Parse(data)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if !result.IsValid() {
//	    for _, e := range result.Errors {
//	        fmt.Printf("%s: %s\n", e.Field, e.Message)
//	    }
//	}
//	af := result.Agentfile
package agentfile
