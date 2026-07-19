package limiter

import (
	_ "embed"
)

//go:embed token_bucket.lua
var tokenBucketScript string

//go:embed gcra.lua
var gcraScript string

// ScriptContent returns the embedded Lua script for the token bucket algorithm.
func ScriptContent() string {
	return tokenBucketScript
}

// GCRAScriptContent returns the embedded Lua script for the GCRA algorithm.
func GCRAScriptContent() string {
	return gcraScript
}
