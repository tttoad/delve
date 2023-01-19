package dap

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Launch debug sessions support the following modes:
//
//	-- [DEFAULT] "debug" - builds and launches debugger for specified program (similar to 'dlv debug')
//
//	   Required args: program
//	   Optional args with default: output, cwd, noDebug
//	   Optional args: buildFlags, args
//
//	-- "test" - builds and launches debugger for specified test (similar to 'dlv test')
//
//	   same args as above
//
//	-- "exec" - launches debugger for precompiled binary (similar to 'dlv exec')
//
//	   Required args: program
//	   Optional args with default: cwd, noDebug
//	   Optional args: args
//
//	-- "replay" - replays a trace generated by mozilla rr. Mozilla rr must be installed.
//
//	   Required args: traceDirPath
//	   Optional args: args
//
//	-- "core" - examines a core dump (only supports linux and windows core dumps).
//
//	   Required args: program, coreFilePath
//	   Optional args: args
//
// TODO(hyangah): change this to 'validateLaunchMode' that checks
// all the required/optional fields mentioned above.
func isValidLaunchMode(mode string) bool {
	switch mode {
	case "exec", "debug", "test", "replay", "core":
		return true
	}
	return false
}

// Default values for Launch/Attach configs.
// Used to initialize configuration variables before decoding
// arguments in launch/attach requests.
var (
	defaultLaunchAttachCommonConfig = LaunchAttachCommonConfig{
		Backend:         "default",
		StackTraceDepth: 50,
	}
	defaultLaunchConfig = LaunchConfig{
		Mode:                     "debug",
		LaunchAttachCommonConfig: defaultLaunchAttachCommonConfig,
	}
	defaultAttachConfig = AttachConfig{
		Mode:                     "local",
		LaunchAttachCommonConfig: defaultLaunchAttachCommonConfig,
	}
)

// LaunchConfig is the collection of launch request attributes recognized by DAP implementation.
type LaunchConfig struct {
	// Acceptable values are:
	//   "debug": compiles your program with optimizations disabled, starts and attaches to it.
	//   "test": compiles your unit test program with optimizations disabled, starts and attaches to it.
	//   "exec": executes a precompiled binary and begins a debug session.
	//   "replay": replays an rr trace.
	//   "core": examines a core dump.
	//
	// Default is "debug".
	Mode string `json:"mode,omitempty"`

	// Path to the program folder (or any go file within that folder)
	// when in `debug` or `test` mode, and to the pre-built binary file
	// to debug in `exec` mode.
	// If it is not an absolute path, it will be interpreted as a path
	// relative to Delve's working directory.
	// Required when mode is `debug`, `test`, `exec`, and `core`.
	Program string `json:"program,omitempty"`

	// Command line arguments passed to the debugged program.
	// Relative paths used in Args will be interpreted as paths relative
	// to `cwd`.
	Args []string `json:"args,omitempty"`

	// Working directory of the program being debugged.
	// If a relative path is provided, it will be interpreted as
	// a relative path to Delve's working directory. This is
	// similar to `dlv --wd` flag.
	//
	// If not specified or empty, Delve's working directory is
	// used by default. But for `test` mode, Delve tries to find
	// the test's package source directory and run tests from there.
	// This matches the behavior of `dlv test` and `go test`.
	Cwd string `json:"cwd,omitempty"`

	// Build flags, to be passed to the Go compiler.
	// Relative paths used in BuildFlags will be interpreted as paths
	// relative to Delve's current working directory.
	//
	// It is like `dlv --build-flags`. For example,
	//    "buildFlags": "-tags=integration -mod=vendor -cover -v"
	BuildFlags string `json:"buildFlags,omitempty"`

	// Output path for the binary of the debugee.
	// Relative path is interpreted as the path relative to
	// the Delve's current working directory.
	// This is deleted after the debug session ends.
	Output string `json:"output,omitempty"`

	// NoDebug is used to run the program without debugging.
	NoDebug bool `json:"noDebug,omitempty"`

	// TraceDirPath is the trace directory path for replay mode.
	// Relative path is interpreted as a path relative to Delve's
	// current working directory.
	// This is required for "replay" mode but unused in other modes.
	TraceDirPath string `json:"traceDirPath,omitempty"`

	// CoreFilePath is the core file path for core mode.
	//
	// This is required for "core" mode but unused in other modes.
	CoreFilePath string `json:"coreFilePath,omitempty"`

	// DlvCwd is the new working directory for Delve server.
	// If specified, the server will change its working
	// directory to the specified directory using os.Chdir.
	// Any other launch attributes with relative paths interpreted
	// using Delve's working directory will use this new directory.
	// When Delve needs to build the program (in debug/test modes),
	// it will run the go command from this directory as well.
	//
	// If a relative path is provided as DlvCwd, it will be
	// interpreted as a path relative to Delve's current working
	// directory.
	DlvCwd string `json:"dlvCwd,omitempty"`

	// Env specifies optional environment variables for Delve server
	// in addition to the environment variables Delve initially
	// started with.
	// Variables with 'nil' values can be used to unset the named
	// environment variables.
	// Values are interpreted verbatim. Variable substitution or
	// reference to other environment variables is not supported.
	Env map[string]*string `json:"env,omitempty"`

	OutputModel string `json:"outputModel,omitempty"`
	LaunchAttachCommonConfig
}

// LaunchAttachCommonConfig is the attributes common in both launch/attach requests.
type LaunchAttachCommonConfig struct {
	// Automatically stop program after launch or attach.
	StopOnEntry bool `json:"stopOnEntry,omitempty"`

	// Backend used for debugging. See `dlv backend` for allowed values.
	// Default is "default".
	Backend string `json:"backend,omitempty"`

	// Maximum depth of stack trace to return.
	// Default is 50.
	StackTraceDepth int `json:"stackTraceDepth,omitempty"`

	// Boolean value to indicate whether global package variables
	// should be shown in the variables pane or not.
	ShowGlobalVariables bool `json:"showGlobalVariables,omitempty"`

	// Boolean value to indicate whether registers should be shown
	// in the variables pane or not.
	ShowRegisters bool `json:"showRegisters,omitempty"`

	// Boolean value to indicate whether system goroutines
	// should be hidden from the call stack view.
	HideSystemGoroutines bool `json:"hideSystemGoroutines,omitempty"`

	// String value to indicate which system goroutines should be
	// shown in the call stack view. See filtering documentation:
	// https://github.com/go-delve/delve/blob/master/Documentation/cli/README.md#goroutines
	GoroutineFilters string `json:"goroutineFilters,omitempty"`

	// An array of mappings from a local path (client) to the remote path (debugger).
	// This setting is useful when working in a file system with symbolic links,
	// running remote debugging, or debugging an executable compiled externally.
	// The debug adapter will replace the local path with the remote path in all of the calls.
	SubstitutePath []SubstitutePath `json:"substitutePath,omitempty"`
}

// SubstitutePath defines a mapping from a local path to the remote path.
// Both 'from' and 'to' must be specified and non-null.
// Empty values can be used to add or remove absolute path prefixes when mapping.
// For example, mapping with empy 'to' can be used to work with binaries with trimmed paths.
type SubstitutePath struct {
	// The local path to be replaced when passing paths to the debugger.
	From string `json:"from,omitempty"`
	// The remote path to be replaced when passing paths back to the client.
	To string `json:"to,omitempty"`
}

func (m *SubstitutePath) UnmarshalJSON(data []byte) error {
	// use custom unmarshal to check if both from/to are set.
	type tmpType struct {
		From *string
		To   *string
	}
	var tmp tmpType

	if err := json.Unmarshal(data, &tmp); err != nil {
		if _, ok := err.(*json.UnmarshalTypeError); ok {
			return fmt.Errorf(`cannot use %s as 'substitutePath' of type {"from":string, "to":string}`, data)
		}
		return err
	}
	if tmp.From == nil || tmp.To == nil {
		return errors.New("'substitutePath' requires both 'from' and 'to' entries")
	}
	*m = SubstitutePath{*tmp.From, *tmp.To}
	return nil
}

// AttachConfig is the collection of attach request attributes recognized by DAP implementation.
type AttachConfig struct {
	// Acceptable values are:
	//   "local": attaches to the local process with the given ProcessID.
	//   "remote": expects the debugger to already be running to "attach" to an in-progress debug session.
	//
	// Default is "local".
	Mode string `json:"mode"`

	// The numeric ID of the process to be debugged. Required and must not be 0.
	ProcessID int `json:"processId,omitempty"`

	LaunchAttachCommonConfig
}

// unmarshalLaunchAttachArgs wraps unmarshalling of launch/attach request's
// arguments attribute. Upon unmarshal failure, it returns an error massaged
// to be suitable for end-users.
func unmarshalLaunchAttachArgs(input json.RawMessage, config interface{}) error {
	if err := json.Unmarshal(input, config); err != nil {
		if uerr, ok := err.(*json.UnmarshalTypeError); ok {
			// Format json.UnmarshalTypeError error string in our own way. E.g.,
			//   "json: cannot unmarshal number into Go struct field LaunchArgs.substitutePath of type dap.SubstitutePath"
			//   => "cannot unmarshal number into 'substitutePath' of type {from:string, to:string}"
			//   "json: cannot unmarshal number into Go struct field LaunchArgs.program of type string" (go1.16)
			//   => "cannot unmarshal number into 'program' of type string"
			typ := uerr.Type.String()
			if uerr.Field == "substitutePath" {
				typ = `{"from":string, "to":string}`
			}
			return fmt.Errorf("cannot unmarshal %v into %q of type %v", uerr.Value, uerr.Field, typ)
		}
		return err
	}
	return nil
}

func prettyPrint(config interface{}) string {
	pretty, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return fmt.Sprintf("%#v", config)
	}
	return string(pretty)
}
