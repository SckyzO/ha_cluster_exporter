package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"

	// cannot use as setConfigDefault function will not work here
	// log.level and log.format flags are set in vars/init
	// "github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"

	"github.com/spf13/viper"
	// we could use this but want to define our own defaults
	// webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"github.com/alecthomas/kingpin/v2"

	"github.com/ClusterLabs/ha_cluster_exporter/collector"
	"github.com/ClusterLabs/ha_cluster_exporter/collector/corosync"
	"github.com/ClusterLabs/ha_cluster_exporter/collector/drbd"
	"github.com/ClusterLabs/ha_cluster_exporter/collector/pacemaker"
	"github.com/ClusterLabs/ha_cluster_exporter/collector/sbd"
)

const (
	namespace = "ha_cluster_exporter"
)

var (
	config *viper.Viper

	// general flags
	webListenAddress *string
	webTelemetryPath *string
	webConfig        *string
	logLevel         *string
	logFormat        *string

	// collector flags
	haClusterCrmMonPath              *string
	haClusterCibadminPath            *string
	haClusterCorosyncCfgtoolpathPath *string
	haClusterCorosyncQuorumtoolPath  *string
	haClusterSbdPath                 *string
	haClusterSbdConfigPath           *string
	haClusterDrbdsetupPath           *string
	haClusterDrbdsplitbrainPath      *string

	// collector enable flags
	enablePacemakerCollector *bool
	enableCorosyncCollector  *bool
	enableSbdCollector       *bool
	enableDrbdCollector      *bool
	commandTimeout           *time.Duration

	promlogConfig = &promlog.Config{
		Level:  &promlog.AllowedLevel{},
		Format: &promlog.AllowedFormat{},
	}
)

func init() {
	config = viper.New()
	config.SetConfigName("ha_cluster_exporter")
	config.AddConfigPath("./")
	config.AddConfigPath("$HOME/.config/")
	config.AddConfigPath("/etc/")
	config.AddConfigPath("/usr/etc/")
	config.ReadInConfig()

	// general flags
	webListenAddress = kingpin.Flag(
		"web.listen-address",
		"Address to listen on for web interface and telemetry.",
	).PlaceHolder(":9664").Default(setConfigDefault("web.listen-address", ":9664")).String()
	webTelemetryPath = kingpin.Flag(
		"web.telemetry-path",
		"Path under which to expose metrics.",
	).PlaceHolder("/metrics").Default(setConfigDefault("web.telemetry-path", "/metrics")).String()
	// we could use this but want to define our own defaults
	// webConfig = webflag.AddFlags(kingpin.CommandLine)
	webConfig = kingpin.Flag(
		"web.config.file",
		"[EXPERIMENTAL] Path to configuration file that can enable TLS or authentication.",
	).PlaceHolder("/etc/" + namespace + ".web.yaml").Default(setConfigDefault("web.config.file", "/etc/"+namespace+".web.yaml")).String()

	// collector flags
	haClusterCrmMonPath = kingpin.Flag(
		"crm-mon-path",
		"path to crm_mon executable",
	).PlaceHolder("/usr/sbin/crm_mon").Default(setConfigDefault("crm-mon-path", "/usr/sbin/crm_mon")).String()
	haClusterCibadminPath = kingpin.Flag(
		"cibadmin-path",
		"path to cibadmin executable",
	).PlaceHolder("/usr/sbin/cibadmin").Default(setConfigDefault("cibadmin-path", "/usr/sbin/cibadmin")).String()
	haClusterCorosyncCfgtoolpathPath = kingpin.Flag(
		"corosync-cfgtoolpath-path",
		"path to corosync-cfgtool executable",
	).PlaceHolder("/usr/sbin/corosync-cfgtool").Default(setConfigDefault("corosync-cfgtoolpath-path", "/usr/sbin/corosync-cfgtool")).String()
	haClusterCorosyncQuorumtoolPath = kingpin.Flag(
		"corosync-quorumtool-path",
		"path to corosync-quorumtool executable",
	).PlaceHolder("/usr/sbin/corosync-quorumtool").Default(setConfigDefault("corosync-quorumtool-path", "/usr/sbin/corosync-quorumtool")).String()
	haClusterSbdPath = kingpin.Flag(
		"sbd-path",
		"path to sbd executable",
	).PlaceHolder("/usr/sbin/sbd").Default(setConfigDefault("sbd-path", "/usr/sbin/sbd")).String()
	haClusterSbdConfigPath = kingpin.Flag(
		"sbd-config-path",
		"path to sbd configuration",
	).PlaceHolder("/etc/sysconfig/sbd").Default(setConfigDefault("sbd-config-path", "/etc/sysconfig/sbd")).String()
	haClusterDrbdsetupPath = kingpin.Flag(
		"drbdsetup-path",
		"path to drbdsetup executable",
	).PlaceHolder("/sbin/drbdsetup").Default(setConfigDefault("drbdsetup-path", "/sbin/drbdsetup")).String()
	haClusterDrbdsplitbrainPath = kingpin.Flag(
		"drbdsplitbrain-path",
		"path to drbd splitbrain hooks temporary files",
	).PlaceHolder("/var/run/drbd/splitbrain").Default(setConfigDefault("drbdsplitbrain-path", "/var/run/drbd/splitbrain")).String()

	enablePacemakerCollector = kingpin.Flag(
		"collector.pacemaker",
		"Enable the Pacemaker collector (default: enabled).",
	).Default(setConfigDefault("collector.pacemaker", "true")).Bool()
	enableCorosyncCollector = kingpin.Flag(
		"collector.corosync",
		"Enable the Corosync collector (default: enabled).",
	).Default(setConfigDefault("collector.corosync", "true")).Bool()
	enableSbdCollector = kingpin.Flag(
		"collector.sbd",
		"Enable the SBD collector (default: enabled).",
	).Default(setConfigDefault("collector.sbd", "true")).Bool()
	enableDrbdCollector = kingpin.Flag(
		"collector.drbd",
		"Enable the DRBD collector (default: enabled).",
	).Default(setConfigDefault("collector.drbd", "true")).Bool()

	commandTimeout = kingpin.Flag(
		"collector.timeout",
		"Timeout for system commands execution",
	).PlaceHolder("10s").Default(setConfigDefault("collector.timeout", "10s")).Duration()

	// cannot use as setConfigDefault function will not work here
	// log.level and log.format flags are set in vars/init
	// flag.AddFlags(kingpin.CommandLine, promlogConfig)
	logLevel = kingpin.Flag(
		"log.level",
		"Only log messages with the given severity or above. One of: [debug, info, warn, error]",
	).PlaceHolder("info").Default(setConfigDefault("log.level", "info")).String()
	logFormat = kingpin.Flag(
		"log.format",
		"Output format of log messages. One of: [logfmt, json]",
	).PlaceHolder("logfmt").Default(setConfigDefault("log.format", "logfmt")).String()

	// detect unit testing and skip kingpin.Parse() in init.
	// see: https://github.com/alecthomas/kingpin/issues/187
	testing := (strings.HasSuffix(os.Args[0], ".test") ||
		strings.HasSuffix(os.Args[0], "__debug_bin"))
	if testing {
		return
	}

	kingpin.Version(version.Print(namespace))
	kingpin.HelpFlag.Short('h')

	var err error

	kingpin.Parse()

	err = promlogConfig.Level.Set(*logLevel)
	if err != nil {
		fmt.Printf("%s: error: %s, try --help\n", namespace, err)
		os.Exit(1)
	}
	err = promlogConfig.Format.Set(*logFormat)
	if err != nil {
		fmt.Printf("%s: error: %s, try --help\n", namespace, err)
		os.Exit(1)
	}
}

// looks up if a configName is define in viper config
// if it is not defined in the viper config, set the passed configDefault
func setConfigDefault(configName string, configDefault string) string {
	var result string
	if config.IsSet(configName) {
		result = config.GetString(configName)
	} else {
		result = configDefault
	}
	return result
}

func registerCollectors(logger log.Logger) (collectors []prometheus.Collector, errors []error) {
	if *enablePacemakerCollector {
		pacemakerCollector, err := pacemaker.NewCollector(
			*haClusterCrmMonPath,
			*haClusterCibadminPath,
			*commandTimeout,
			logger,
		)
		if err != nil {
			errors = append(errors, err)
		} else {
			collectors = append(collectors, pacemakerCollector)
		}
	}

	if *enableCorosyncCollector {
		corosyncCollector, err := corosync.NewCollector(
			*haClusterCorosyncCfgtoolpathPath,
			*haClusterCorosyncQuorumtoolPath,
			*commandTimeout,
			logger,
		)
		if err != nil {
			errors = append(errors, err)
		} else {
			collectors = append(collectors, corosyncCollector)
		}
	}

	if *enableSbdCollector {
		sbdCollector, err := sbd.NewCollector(
			*haClusterSbdPath,
			*haClusterSbdConfigPath,
			*commandTimeout,
			logger,
		)
		if err != nil {
			errors = append(errors, err)
		} else {
			collectors = append(collectors, sbdCollector)
		}
	}

	if *enableDrbdCollector {
		drbdCollector, err := drbd.NewCollector(
			*haClusterDrbdsetupPath,
			*haClusterDrbdsplitbrainPath,
			*commandTimeout,
			logger,
		)
		if err != nil {
			errors = append(errors, err)
		} else {
			collectors = append(collectors, drbdCollector)
		}
	}

	for i, c := range collectors {
		if c, ok := c.(collector.InstrumentableCollector); ok {
			collectors[i] = collector.NewInstrumentedCollector(c, logger)
		}
	}

	prometheus.MustRegister(collectors...)

	return collectors, errors
}

func main() {
	var err error

	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", fmt.Sprintf("Starting %s %s", namespace, version.Info()))
	level.Info(logger).Log("msg", fmt.Sprintf("Build context %s", version.BuildContext()))

	// re-read only to display Info/Warn
	err = config.ReadInConfig()
	if err != nil {
		level.Warn(logger).Log("msg", "Reading config file failed", "err", err)
		level.Info(logger).Log("msg", "Default config values will be used")
	} else {
		level.Info(logger).Log("msg", "Using config file: "+config.ConfigFileUsed())
	}

	// register collectors
	collectors, errors := registerCollectors(logger)
	for _, err = range errors {
		level.Warn(logger).Log("msg", "Registration failure", "err", err)
	}
	if len(collectors) == 0 {
		level.Error(logger).Log("msg", "No collector could be registered.")
		os.Exit(1)
	}
	for _, c := range collectors {
		if c, ok := c.(collector.SubsystemCollector); ok {
			level.Info(logger).Log("msg", c.GetSubsystem()+" collector registered.")
		}
	}

	// if we're not in debug log level, we unregister the Go runtime metrics collector that gets registered by default
	if *logLevel != "debug" {
		prometheus.Unregister(prometheus.NewGoCollector())
	}

	fullListenAddress := *webListenAddress
	serveAddress := &http.Server{Addr: fullListenAddress}
	servePath := *webTelemetryPath

	landingPage := []byte(`<html>
<head>
	<title>ClusterLabs Linux HA Cluster Exporter</title>
</head>
<body>
	<h1>ClusterLabs Linux HA Cluster Exporter</h1>
	<h2>Prometheus exporter for Pacemaker based Linux HA clusters</h2>
	<ul>
		<li><a href="` + servePath + `">Metrics</a></li>
		<li><a href="https://github.com/ClusterLabs/ha_cluster_exporter" target="_blank">GitHub</a></li>
	</ul>
</body>
</html>
`)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(landingPage)
	})
	http.Handle(servePath, promhttp.Handler())

	level.Info(logger).Log("msg", "Serving metrics on "+fullListenAddress+servePath)

	toolkitFlags := &web.FlagConfig{
		WebListenAddresses: func() *[]string {
			r := []string{*webListenAddress}
			return &r
		}(),
		WebSystemdSocket: func() *bool {
			r := false
			return &r
		}(),
		WebConfigFile: func() *string {
			r := ""
			return &r
		}(),
	}

	var listen error
	_, err = os.Stat(*webConfig)

	if err != nil {
		if *webConfig == "/etc/"+namespace+".web.yaml" && os.IsNotExist(err) {
			level.Info(logger).Log("msg", "Web config file not found at default location, using default web settings")
		} else {
			level.Warn(logger).Log("msg", "Reading web config file failed", "err", err)
			level.Info(logger).Log("msg", "Default web config or commandline values will be used")
		}
		listen = web.ListenAndServe(serveAddress, toolkitFlags, logger)
	} else {
		level.Info(logger).Log("msg", "Using web config file: "+*webConfig)
		toolkitFlags.WebConfigFile = webConfig
		listen = web.ListenAndServe(serveAddress, toolkitFlags, logger)
	}

	if err := listen; err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
