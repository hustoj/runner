package runner

import (
	"go.uber.org/zap"
)

func InitLogger(logPath string, debug bool) *zap.SugaredLogger {
	var cfg zap.Config
	if debug {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}


	if len(logPath) > 0 {
		cfg.OutputPaths = []string{logPath}
	} else {
		cfg.OutputPaths = []string{"stderr"}
	}

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}

	log = logger.Sugar()
	return log
}
