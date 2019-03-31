package clicommand

import (
	"github.com/buildkite/agent/agent"
	"github.com/buildkite/agent/cliconfig"
	"github.com/urfave/cli"
)

var DownloadHelpDescription = `Usage:

   buildkite-agent artifact download [arguments...]

Description:

   Downloads artifacts from Buildkite to the local machine.

   Note: You need to ensure that your search query is surrounded by quotes if
   using a wild card as the built-in shell path globbing will provide files,
   which will break the download.

Example:

   $ buildkite-agent artifact download "pkg/*.tar.gz" . --build xxx

   This will search across all the artifacts for the build with files that match that part.
   The first argument is the search query, and the second argument is the download destination.

   If you're trying to download a specific file, and there are multiple artifacts from different
   jobs, you can target the particular job you want to download the artifact from:

   $ buildkite-agent artifact download "pkg/*.tar.gz" . --step "tests" --build xxx

   You can also use the step's jobs id (provided by the environment variable $BUILDKITE_JOB_ID)`

type ArtifactDownloadConfig struct {
	Query       string `cli:"arg:0" label:"artifact search query" validate:"required"`
	Destination string `cli:"arg:1" label:"artifact download path" validate:"required"`
	Step        string `cli:"step"`
	Build       string `cli:"build" validate:"required"`

	// Global flags
	Debug   bool `cli:"debug"`
	NoColor bool `cli:"no-color"`

	// API config
	DebugHTTP        bool   `cli:"debug-http"`
	AgentAccessToken string `cli:"agent-access-token" validate:"required"`
	Endpoint         string `cli:"endpoint" validate:"required"`
	NoHTTP2          bool   `cli:"no-http2"`
}

var ArtifactDownloadCommand = cli.Command{
	Name:        "download",
	Usage:       "Downloads artifacts from Buildkite to the local machine",
	Description: DownloadHelpDescription,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "step",
			Value: "",
			Usage: "Scope the search to a paticular step by using either it's name or job ID",
		},
		cli.StringFlag{
			Name:   "build",
			Value:  "",
			EnvVar: "BUILDKITE_BUILD_ID",
			Usage:  "The build that the artifacts were uploaded to",
		},

		// API Flags
		AgentAccessTokenFlag,
		EndpointFlag,
		NoHTTP2Flag,
		DebugHTTPFlag,

		// Global flags
		NoColorFlag,
		DebugFlag,
	},
	Action: func(c *cli.Context) {
		// The configuration will be loaded into this struct
		cfg := ArtifactDownloadConfig{}

		l := CreateLogger(&cfg)

		// Load the configuration
		if err := cliconfig.Load(c, l, &cfg); err != nil {
			l.Fatal("%s", err)
		}

		// Setup the any global configuration options
		HandleGlobalFlags(l, cfg)

		// Create the API client
		client := agent.NewAPIClient(l, loadAPIClientConfig(cfg, `AgentAccessToken`))

		// Setup the downloader
		downloader := agent.NewArtifactDownloader(l, client, agent.ArtifactDownloaderConfig{
			Query:       cfg.Query,
			Destination: cfg.Destination,
			BuildID:     cfg.Build,
			Step:        cfg.Step,
		})

		// Download the artifacts
		if err := downloader.Download(); err != nil {
			l.Fatal("Failed to download artifacts: %s", err)
		}
	},
}
