package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "openvox-code",
	Short: "Fast, Git-native Puppet environment deployment tool",
	Long: `openvox-code is a fast, Git-native Puppet environment deployment tool written in Go.

It replaces r10k and g10k with a simpler, more focused approach:
parallel Git fetches, bare clone caching, atomic deploys, and OCI image output.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("openvox-code %s\n", version)
		fmt.Printf("  commit:  %s\n", commit)
		fmt.Printf("  built:   %s\n", date)
		fmt.Printf("  go:      %s\n", runtime.Version())
		fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

// Global flags
var (
	cfgFile        string
	cacheDir       string
	environmentDir string
	lockfilePath   string
	verbose        bool
	quiet          bool
	parallel       int
	outputFormat   string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "openvox-code.yaml", "path to configuration file")
	rootCmd.PersistentFlags().StringVar(&cacheDir, "cachedir", "", "override cache directory")
	rootCmd.PersistentFlags().StringVar(&environmentDir, "environmentdir", "", "override environment directory")
	rootCmd.PersistentFlags().StringVar(&lockfilePath, "lockfile", "", "path to lockfile")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-error output")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "output format (text, json)")
	rootCmd.PersistentFlags().IntVar(&parallel, "parallel", runtime.NumCPU(), "max parallel Git operations")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(mirrorCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(lockCmd)
}

// Exit codes for openvox-code.
const (
	ExitOK            = 0
	ExitGenericError  = 1
	ExitConfigError   = 2
	ExitGitError      = 3
	ExitDeployError   = 4
	ExitBuildError    = 5
	ExitValidateError = 6
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(ExitGenericError)
	}
}
