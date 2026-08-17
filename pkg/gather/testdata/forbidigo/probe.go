package forbidigo

import (
	stdlog "log"
	"os"

	"go.uber.org/zap"
)

func forbidden(log *zap.SugaredLogger, logger *zap.Logger) {
	os.Exit(1)
	stdlog.Fatal("x")
	stdlog.Fatalf("x")
	stdlog.Fatalln("x")
	logger.Fatal("x")
	log.Fatal("x")
	log.Fatalf("x")
	log.Fatalln("x")
	log.Fatalw("x")
}
