package app

import (
	"strings"

	"github.com/rs/zerolog"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

var _ tmlog.Logger = (*ThornodeLogWrapper)(nil)

// ThornodeLogWrapper provides a wrapper around a zerolog.Logger instance. It implements
// Tendermint's Logger interface.
type ThornodeLogWrapper struct {
	zerolog.Logger
	ExcludeModules []string
}

// Info implements Tendermint's Logger interface and logs with level INFO. A set
// of key/value tuples may be provided to add context to the log. The number of
// tuples must be even and the key of the tuple must be a string.
func (z ThornodeLogWrapper) Info(msg string, keyVals ...interface{}) {
	z.Logger.Info().Fields(getLogFields(keyVals...)).Msg(msg)
}

// Error implements Tendermint's Logger interface and logs with level ERR. A set
// of key/value tuples may be provided to add context to the log. The number of
// tuples must be even and the key of the tuple must be a string.
func (z ThornodeLogWrapper) Error(msg string, keyVals ...interface{}) {
	z.Logger.Error().Fields(getLogFields(keyVals...)).Msg(msg)
}

// Debug implements Tendermint's Logger interface and logs with level DEBUG. A set
// of key/value tuples may be provided to add context to the log. The number of
// tuples must be even and the key of the tuple must be a string.
func (z ThornodeLogWrapper) Debug(msg string, keyVals ...interface{}) {
	z.Logger.Debug().Fields(getLogFields(keyVals...)).Msg(msg)
}

// With returns a new wrapped logger with additional context provided by a set
// of key/value tuples. The number of tuples must be even and the key of the
// tuple must be a string.
func (z ThornodeLogWrapper) With(keyVals ...interface{}) tmlog.Logger {
	if len(keyVals)%2 != 0 {
		return ThornodeLogWrapper{
			Logger:         z.Logger.With().Fields(getLogFields(keyVals...)).Logger(),
			ExcludeModules: z.ExcludeModules,
		}
	}
	for i := 0; i < len(keyVals); i += 2 {
		name, ok := keyVals[i].(string)
		if !ok {
			z.Logger.Error().Interface("key", keyVals[i]).Msg("non-string logging key provided")
		}
		if name != "module" {
			continue
		}
		value, ok := keyVals[i+1].(string)
		if !ok {
			continue
		}
		for _, item := range z.ExcludeModules {
			if strings.EqualFold(item, value) {
				return ThornodeLogWrapper{
					Logger:         z.Logger.Level(zerolog.WarnLevel).With().Fields(getLogFields(keyVals...)).Logger(),
					ExcludeModules: z.ExcludeModules,
				}
			}
		}
	}
	return ThornodeLogWrapper{
		Logger:         z.Logger.With().Fields(getLogFields(keyVals...)).Logger(),
		ExcludeModules: z.ExcludeModules,
	}
}

func getLogFields(keyVals ...interface{}) map[string]interface{} {
	if len(keyVals)%2 != 0 {
		return nil
	}

	fields := make(map[string]interface{})
	for i := 0; i < len(keyVals); i += 2 {
		val, ok := keyVals[i].(string)
		if ok {
			fields[val] = keyVals[i+1]
		}
	}

	return fields
}
