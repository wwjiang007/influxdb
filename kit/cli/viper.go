package cli

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/influxdata/influxdb/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Opt is a single command-line option
type Opt struct {
	DestP interface{} // pointer to the destination

	EnvVar     string
	Flag       string
	Hidden     bool
	Persistent bool
	Required   bool
	Short      rune // using rune b/c it guarantees correctness. a short must always be a string of length 1

	Default interface{}
	Desc    string
}

// NewOpt creates a new command line option.
func NewOpt(destP interface{}, flag string, dflt interface{}, desc string) Opt {
	return Opt{
		DestP:   destP,
		Flag:    flag,
		Default: dflt,
		Desc:    desc,
	}
}

// Program parses CLI options
type Program struct {
	// Run is invoked by cobra on execute.
	Run func() error
	// Name is the name of the program in help usage and the env var prefix.
	Name string
	// Opts are the command line/env var options to the program
	Opts []Opt
}

// NewCommand creates a new cobra command to be executed that respects env vars.
//
// Uses the upper-case version of the program's name as a prefix
// to all environment variables.
//
// This is to simplify the viper/cobra boilerplate.
func NewCommand(v *viper.Viper, p *Program) *cobra.Command {
	cmd := &cobra.Command{
		Use:  p.Name,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return p.Run()
		},
	}

	v.SetEnvPrefix(strings.ToUpper(p.Name))
	v.AutomaticEnv()
	// This normalizes "-" to an underscore in env names.
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	if configPath := v.GetString("CONFIG_PATH"); configPath != "" {
		switch path.Ext(configPath) {
		case ".json", ".toml", ".yaml", "yml":
			v.SetConfigFile(configPath)
		case "":
			v.AddConfigPath(configPath)
		}
	} else {
		// defaults to looking in same directory as program running for
		// a file with base `config` and extensions .json|.toml|.yaml|.yml
		v.SetConfigName("config")
		v.AddConfigPath(".")
	}

	// done before we bind flags to viper keys.
	// order of precedence (1 highest -> 3 lowest):
	//	1. flags
	//  2. env vars
	//	3. config file
	if err := initializeConfig(v); err != nil {
		panic("invalid config file caused panic: " + err.Error())
	}
	BindOptions(v, cmd, p.Opts)

	return cmd
}

func initializeConfig(v *viper.Viper) error {
	err := v.ReadInConfig()
	if err != nil && !os.IsNotExist(err) {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}
	return nil
}

// BindOptions adds opts to the specified command and automatically
// registers those options with viper.
func BindOptions(v *viper.Viper, cmd *cobra.Command, opts []Opt) {
	for _, o := range opts {
		flagset := cmd.Flags()
		if o.Persistent {
			flagset = cmd.PersistentFlags()
		}

		if o.Required {
			cmd.MarkFlagRequired(o.Flag)
		}

		envVar := o.Flag
		if o.EnvVar != "" {
			envVar = o.EnvVar
		}

		hasShort := o.Short != 0

		switch destP := o.DestP.(type) {
		case *string:
			var d string
			if o.Default != nil {
				d = o.Default.(string)
			}
			if hasShort {
				flagset.StringVarP(destP, o.Flag, string(o.Short), d, o.Desc)
			} else {
				flagset.StringVar(destP, o.Flag, d, o.Desc)
			}
			mustBindPFlag(v, o.Flag, flagset)
			*destP = v.GetString(envVar)
		case *int:
			var d int
			if o.Default != nil {
				d = o.Default.(int)
			}
			if hasShort {
				flagset.IntVarP(destP, o.Flag, string(o.Short), d, o.Desc)
			} else {
				flagset.IntVar(destP, o.Flag, d, o.Desc)
			}
			mustBindPFlag(v, o.Flag, flagset)
			*destP = v.GetInt(envVar)
		case *int32:
			var d int32
			if o.Default != nil {
				// N.B. since our CLI kit types default values as interface{} and
				// literal numbers get typed as int by default, it's very easy to
				// create an int32 CLI flag with an int default value.
				//
				// The compiler doesn't know to complain in that case, so you end up
				// with a runtime panic when trying to bind the CLI options.
				//
				// To avoid that headache, we support both int32 and int defaults
				// for int32 fields. This introduces a new runtime bomb if somebody
				// specifies an int default > math.MaxInt32, but that's hopefully
				// less likely.
				var ok bool
				d, ok = o.Default.(int32)
				if !ok {
					d = int32(o.Default.(int))
				}
			}
			if hasShort {
				flagset.Int32VarP(destP, o.Flag, string(o.Short), d, o.Desc)
			} else {
				flagset.Int32Var(destP, o.Flag, d, o.Desc)
			}
			mustBindPFlag(v, o.Flag, flagset)
			*destP = v.GetInt32(envVar)
		case *int64:
			var d int64
			if o.Default != nil {
				// N.B. since our CLI kit types default values as interface{} and
				// literal numbers get typed as int by default, it's very easy to
				// create an int64 CLI flag with an int default value.
				//
				// The compiler doesn't know to complain in that case, so you end up
				// with a runtime panic when trying to bind the CLI options.
				//
				// To avoid that headache, we support both int64 and int defaults
				// for int64 fields.
				var ok bool
				d, ok = o.Default.(int64)
				if !ok {
					d = int64(o.Default.(int))
				}
			}
			if hasShort {
				flagset.Int64VarP(destP, o.Flag, string(o.Short), d, o.Desc)
			} else {
				flagset.Int64Var(destP, o.Flag, d, o.Desc)
			}
			mustBindPFlag(v, o.Flag, flagset)
			*destP = v.GetInt64(envVar)
		case *bool:
			var d bool
			if o.Default != nil {
				d = o.Default.(bool)
			}
			if hasShort {
				flagset.BoolVarP(destP, o.Flag, string(o.Short), d, o.Desc)
			} else {
				flagset.BoolVar(destP, o.Flag, d, o.Desc)
			}
			mustBindPFlag(v, o.Flag, flagset)
			*destP = v.GetBool(envVar)
		case *time.Duration:
			var d time.Duration
			if o.Default != nil {
				d = o.Default.(time.Duration)
			}
			if hasShort {
				flagset.DurationVarP(destP, o.Flag, string(o.Short), d, o.Desc)
			} else {
				flagset.DurationVar(destP, o.Flag, d, o.Desc)
			}
			mustBindPFlag(v, o.Flag, flagset)
			*destP = v.GetDuration(envVar)
		case *[]string:
			var d []string
			if o.Default != nil {
				d = o.Default.([]string)
			}
			if hasShort {
				flagset.StringSliceVarP(destP, o.Flag, string(o.Short), d, o.Desc)
			} else {
				flagset.StringSliceVar(destP, o.Flag, d, o.Desc)
			}
			mustBindPFlag(v, o.Flag, flagset)
			*destP = v.GetStringSlice(envVar)
		case *map[string]string:
			var d map[string]string
			if o.Default != nil {
				d = o.Default.(map[string]string)
			}
			if hasShort {
				flagset.StringToStringVarP(destP, o.Flag, string(o.Short), d, o.Desc)
			} else {
				flagset.StringToStringVar(destP, o.Flag, d, o.Desc)
			}
			mustBindPFlag(v, o.Flag, flagset)
			*destP = v.GetStringMapString(envVar)
		case pflag.Value:
			if hasShort {
				flagset.VarP(destP, o.Flag, string(o.Short), o.Desc)
			} else {
				flagset.Var(destP, o.Flag, o.Desc)
			}
			if o.Default != nil {
				destP.Set(o.Default.(string))
			}
			mustBindPFlag(v, o.Flag, flagset)
			destP.Set(v.GetString(envVar))
		case *influxdb.ID:
			var d influxdb.ID
			if o.Default != nil {
				d = o.Default.(influxdb.ID)
			}
			if hasShort {
				IDVarP(flagset, destP, o.Flag, string(o.Short), d, o.Desc)
			} else {
				IDVar(flagset, destP, o.Flag, d, o.Desc)
			}
			if s := v.GetString(envVar); s != "" {
				_ = (*destP).DecodeFromString(v.GetString(envVar))
			}
		default:
			// if you get a panic here, sorry about that!
			// anyway, go ahead and make a PR and add another type.
			panic(fmt.Errorf("unknown destination type %t", o.DestP))
		}

		// so weirdness with the flagset her, the flag must be set before marking it
		// hidden. This is in contrast to the MarkRequired, which can be set before...
		if o.Hidden {
			flagset.MarkHidden(o.Flag)
		}
	}
}

func mustBindPFlag(v *viper.Viper, key string, flagset *pflag.FlagSet) {
	if err := v.BindPFlag(key, flagset.Lookup(key)); err != nil {
		panic(err)
	}
}
