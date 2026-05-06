// Package cli implements the subtrans command-line interface.
package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:           "subtrans [flags] input.srt",
	Short:         "Translate SRT subtitles using OpenAI-compatible APIs",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.MaximumNArgs(1),
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		if verbose {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return runTranslate(args[0])
	},
}

// Execute sets the version and runs the root command.
func Execute(version string) {
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetVersionTemplate("{{printf \"%s version %s\\n\" .Name .Version}}")

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "debug logging")

	f := rootCmd.Flags()
	f.StringVarP(&translateOutput, "output", "o", "", "output SRT file path")
	f.StringVarP(&translateLanguage, "language", "l", "zh", "target language ISO code (e.g. zh, en, ja, ko, fr)")
	f.StringVarP(&translateModel, "model", "m", "", "model override")
	f.StringVar(&translateInstructionsFile, "instructions", "", "path to instructions text file")
	f.StringVar(&translatePrompt, "prompt", "", "custom user prompt prefix")
	f.IntVar(&translateMaxBatchSize, "max-batch-size", 30, "lines per batch")
	f.StringVar(&translateBatchSplitPunct, "batch-split-punct", ".", "punctuation characters for batch splitting")
	f.Float64Var(&translateTemperature, "temperature", 0.0, "LLM temperature")
	f.IntVar(&translateMaxRetries, "max-retries", 3, "API retry count")
	f.BoolVar(&translateIncludeOriginal, "include-original", false, "include original in output")
	f.BoolVar(&translateStripPunctuation, "strip-punctuation", true, "strip trailing periods and commas")

	sf := serveCmd.Flags()
	sf.StringVar(&serveHost, "host", "localhost", "host to listen on")
	sf.IntVarP(&servePort, "port", "p", 8091, "port to listen on")

	rootCmd.AddCommand(serveCmd)
}
