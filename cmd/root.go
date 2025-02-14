package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fabio42/ssl-checker/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	version = "0.1.0"
	logFile = "./ssl-checker.log"
)

var (
	configFile string
	envCheck   string
)

var rootCmd = &cobra.Command{
	Use:     "ssl-checker [flags] [file-targets <files>|domain-targets <domains>]",
	Version: version,
	Short:   "ssl-checker",
	Long:    "ssl-checker is a tool to _quickly_ check certificate details of multiple https targets.",

	Run: func(cmd *cobra.Command, args []string) {
		if viper.GetBool("debug") {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			log.Warn().Msgf("Debug is enabled, log will be found in ./zeroDebug.log")
		}

		envQuery := strings.Split(envCheck, ",")

		if viper.IsSet("queries") {
			queries := viper.Get("queries")
			fileTargets := map[string]string{}
			domainTargets := map[string][]string{}

			for env, data := range queries.(map[string]interface{}) {
				if envCheck != "" && !sliceContains(envQuery, env) {
					continue
				}
				switch data := data.(type) {
				case string:
					fileTargets[env] = data
				case []interface{}:
					domains := make([]string, len(data))
					for k, domain := range data {
						switch domain := domain.(type) {
						case string:
							domains[k] = domain
						default:
							log.Fatal().Msgf("Unsupported data type in query option for %s: %v is of type %T", env, domain, domain)
						}
						domainTargets[env] = domains
					}
				default:
					log.Fatal().Msgf("Unsupported data type in queries option: %v is of type %T", data, data)
				}
			}
			log.Debug().Msgf("fileTargets is  : %v", fileTargets)
			log.Debug().Msgf("domainTargets is: %v", domainTargets)

			runQueries(fileTargets, domainTargets)

		} else {
			// Nothing to do
			log.Debug().Msgf("Empty query... nothing to do")
			os.Exit(1)
		}
	},
}

func runQueries(fileTargets map[string]string, domainTargets map[string][]string) {
	if viper.GetBool("silent") {
		fmt.Fprintln(os.Stderr, "Processing query!")
	}
	q := ui.NewModel(viper.GetInt("timeout"), viper.GetBool("silent"), fileTargets, domainTargets)
	if err := tea.NewProgram(q).Start(); err != nil {
		log.Fatal().Msgf("Error while running TUI program: %v", err)
	}
}

func sliceContains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func init() {
	err := setLogger()
	if err != nil {
		log.Fatal().Msgf("Error failed to configure logger:", err)
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "$HOME/.config/ssl-checker/config.yaml", "Configuration file location")
	cfgFile := filepath.Base(configFile)
	cfgPath := filepath.Dir(configFile)

	viper.SetConfigName(cfgFile[:len(cfgFile)-len(filepath.Ext(cfgFile))])
	viper.AddConfigPath(cfgPath)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Debug().Msg("No config file found")
		} else {
			log.Fatal().Msgf("Error while parsing config: %v", err)
		}
	}

	rootCmd.Flags().StringVarP(&envCheck, "environments", "e", "", "Comma delimited string specifying the environments to check")

	rootCmd.PersistentFlags().BoolP("silent", "s", false, "disable ui")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug log, out will be saved in "+logFile)
	rootCmd.PersistentFlags().Uint16P("timeout", "t", 10, "Set timeout for SSL check queries")

	viper.BindPFlag("silent", rootCmd.PersistentFlags().Lookup("silent"))
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Msgf("Whoops. There was an error while executing your CLI '%s'", err)
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the current version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// https://github.com/rs/zerolog/issues/150
type LevelWriter struct {
	io.Writer
	Level zerolog.Level
}

func (lw *LevelWriter) WriteLevel(l zerolog.Level, p []byte) (n int, err error) {
	if l >= lw.Level {
		return lw.Write(p)
	}
	return len(p), nil
}

func setLogger() error {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if viper.GetBool("debug") {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	logWriter, err := os.OpenFile(
		logFile,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0664,
	)
	if err != nil {
		panic(err)
	}

	fileWriter := zerolog.New(zerolog.ConsoleWriter{
		Out:          logWriter,
		NoColor:      true,
		PartsExclude: []string{"time", "level"},
	})
	consoleWriter := zerolog.NewConsoleWriter(
		func(w *zerolog.ConsoleWriter) {
			w.Out = os.Stderr
			w.PartsExclude = []string{"time"}
		},
	)
	consoleWriterLeveled := &LevelWriter{Writer: consoleWriter, Level: zerolog.InfoLevel}
	log.Logger = zerolog.New(zerolog.MultiLevelWriter(fileWriter, consoleWriterLeveled)).With().Timestamp().Logger()
	return nil
}
