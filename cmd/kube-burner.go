// Copyright 2020 The Kube-burner Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cloud-bulldozer/kube-burner/log"
	"github.com/cloud-bulldozer/kube-burner/pkg/burner"
	"github.com/cloud-bulldozer/kube-burner/pkg/config"

	"github.com/cloud-bulldozer/kube-burner/pkg/indexers"
	"github.com/cloud-bulldozer/kube-burner/pkg/measurements"
	"github.com/cloud-bulldozer/kube-burner/pkg/prometheus"
	"github.com/cloud-bulldozer/kube-burner/pkg/util"

	"github.com/spf13/cobra"
)

var binName = filepath.Base(os.Args[0])

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generates completion scripts for bash shell",
	Long: `To load completion run
	. <(kube-burner completion)
	To configure your bash shell to load completions for each session execute:

	# kube-burner completion > /etc/bash_completion.d/kube-burner
	`,
	Args: cobra.MaximumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenBashCompletion(os.Stdout)
	},
}

func initCmd() *cobra.Command {
	var url, metricsProfile string
	var username, password, uuid, token string
	var skipTLSVerify bool
	var prometheusStep time.Duration
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Launch benchmark",
		Args:  cobra.MaximumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("🔥 Starting kube-burner")
			var p *prometheus.Prometheus
			var err error
			if url != "" {
				p, err = prometheus.NewPrometheusClient(url, token, username, password, metricsProfile, uuid, skipTLSVerify, prometheusStep)
				if err != nil {
					log.Fatalf("Error setting up Prometheus client: %s", err)
				}
			}
			steps(uuid, p, prometheusStep)
		},
	}
	cmd.Flags().StringVar(&uuid, "uuid", "", "Benchmark UUID")
	cmd.Flags().StringVarP(&url, "prometheus-url", "u", "", "Prometheus URL")
	cmd.Flags().StringVarP(&token, "token", "t", "", "Prometheus Bearer token")
	cmd.Flags().StringVar(&username, "username", "", "Prometheus username for authentication")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Prometheus password for basic authentication")
	cmd.Flags().StringVarP(&metricsProfile, "metrics-profile", "m", "metrics.yaml", "Metrics profile file")
	cmd.Flags().BoolVar(&skipTLSVerify, "skip-tls-verify", true, "Verify prometheus TLS certificate")
	cmd.Flags().DurationVarP(&prometheusStep, "step", "s", 30*time.Second, "Prometheus step size")
	cmd.MarkFlagFilename("metrics-profile")
	return cmd
}

func destroyCmd() *cobra.Command {
	var uuid string
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy old namespaces labeled with the given UUID.",
		Args:  cobra.MaximumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			selector := util.NewSelector()
			selector.Configure("", fmt.Sprintf("kube-burner-uuid=%s", uuid), "")
			burner.ReadConfig(0, 0)
			if err := burner.CleanupNamespaces(burner.ClientSet, selector); err != nil {
				log.Fatal(err)
			}
		},
	}
	cmd.Flags().StringVar(&uuid, "uuid", "", "UUID")
	return cmd
}

func indexCmd() *cobra.Command {
	var url, metricsProfile string
	var start, end int64
	var username, password, uuid, token string
	var skipTLSVerify bool
	var prometheusStep time.Duration
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index metrics from the given time range",
		Args:  cobra.MaximumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			var indexer *indexers.Indexer
			if config.ConfigSpec.GlobalConfig.IndexerConfig.Enabled {
				indexer = indexers.NewIndexer(config.ConfigSpec.GlobalConfig.IndexerConfig)
			} else {
				log.Fatal("Indexing is disabled in the configuration")
			}
			p, err := prometheus.NewPrometheusClient(url, token, username, password, metricsProfile, uuid, skipTLSVerify, prometheusStep)
			if err != nil {
				log.Fatal(err)
			}
			startTime := time.Unix(start, 0)
			endTime := time.Unix(end, 0)
			for _, j := range config.ConfigSpec.Jobs {
				p.ScrapeMetrics(startTime, endTime, config.ConfigSpec, j.Name, indexer)
			}
		},
	}
	cmd.Flags().StringVar(&uuid, "uuid", "", "Benchmark UUID")
	cmd.Flags().StringVarP(&url, "prometheus-url", "u", "", "Prometheus URL")
	cmd.Flags().StringVarP(&token, "token", "t", "", "Prometheus Bearer token")
	cmd.Flags().StringVar(&username, "username", "", "Prometheus username for authentication")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Prometheus password for basic authentication")
	cmd.Flags().StringVarP(&metricsProfile, "metrics-profile", "m", "metrics.yaml", "Metrics profile file")
	cmd.Flags().BoolVar(&skipTLSVerify, "skip-tls-verify", true, "Verify prometheus TLS certificate")
	cmd.Flags().DurationVarP(&prometheusStep, "step", "s", 30*time.Second, "Prometheus step size")
	cmd.Flags().Int64VarP(&start, "start", "", time.Now().Unix()-3600, "Epoch start time")
	cmd.Flags().Int64VarP(&end, "end", "", time.Now().Unix(), "Epoch end time")
	cmd.MarkFlagFilename("metrics-profile")
	cmd.MarkFlagRequired("prometheus-url")
	return cmd
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   binName,
	Short: "Stress a kubernetes cluster",
	Long: `kube-burner is a tool that aims to stress a kubernetes cluster.
	
It doesn’t only provide some similar features as other tools like cluster-loader, but also
adds other features such as a simplified simplified usage, metrics collection and indexing capabilities`,
}

// Execute executes rootCmd
func Execute() {
	rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(
		initCmd(),
		destroyCmd(),
		indexCmd(),
	)
	for _, c := range rootCmd.Commands() {
		logLevel := c.Flags().String("log-level", "info", "Allowed values: debug, info, warn, error, fatal")
		configFile := c.Flags().StringP("config", "c", "", "Config file path")
		c.PreRun = func(cmd *cobra.Command, args []string) {
			log.Infof("Setting log level to %s", *logLevel)
			log.SetLogLevel(*logLevel)
			err := config.Parse(*configFile)
			if err != nil {
				log.Fatal(err)
			}
		}
		c.MarkFlagRequired("uuid")
		c.MarkFlagRequired("config")
		c.MarkFlagFilename("config")
	}
	rootCmd.AddCommand(completionCmd)
	cobra.OnInitialize()
}

func steps(uuid string, p *prometheus.Prometheus, prometheusStep time.Duration) {
	verification := true
	var indexer *indexers.Indexer
	if config.ConfigSpec.GlobalConfig.IndexerConfig.Enabled {
		indexer = indexers.NewIndexer(config.ConfigSpec.GlobalConfig.IndexerConfig)
	}
	executorList := burner.NewExecutorList(uuid)
	for _, job := range executorList {
		log.Infof("Triggering job: %s with UUID %s", job.Config.Name, uuid)
		job.Cleanup()
		measurements.NewMeasurementFactory(burner.ClientSet, config.ConfigSpec.GlobalConfig, job.Config, uuid, indexer)
		measurements.Register(config.ConfigSpec.GlobalConfig.Measurements)
		measurements.Start()
		// Run execution
		job.Run()
		measurements.Stop()
		log.Infof("Job %s took %.2f seconds", job.Config.Name, job.End.Sub(job.Start).Seconds())
		if job.Config.VerifyObjects {
			verification = job.Verify()
			// If verification failed and ErrorOnVerify is enabled. Exit with error, otherwise continue
			if !verification && job.Config.ErrorOnVerify {
				log.Fatal("Object verification failed. Exiting")
			}
		}
		measurements.Index()
		// If prometheus is enabled
		if p != nil {
			log.Infof("Waiting extra %v before scraping prometheus metrics", prometheusStep*2)
			time.Sleep(prometheusStep * 2)
			if err := p.ScrapeMetrics(job.Start, job.End.Add(2*prometheusStep), config.ConfigSpec, job.Config.Name, indexer); err != nil {
				log.Error(err)
			}
		}
		// Exit if verification failed
		if job.Config.VerifyObjects && !verification {
			log.Fatal("Object verification failed")
		}
		if job.Config.JobPause > 0 {
			log.Infof("Pausing for %d milliseconds before next job", job.Config.JobPause)
			time.Sleep(time.Millisecond * time.Duration(job.Config.JobPause))
		}
	}
}
