package runner

import "go.uber.org/zap"

var log *zap.SugaredLogger

// SetLogger replaces the package-level logger.
// This is intended for testing; production code should use InitLogger.
func SetLogger(l *zap.SugaredLogger) {
	log = l
}
