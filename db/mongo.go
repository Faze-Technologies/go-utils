package db

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/v2/mongo/otelmongo"
	"go.uber.org/zap"
)

func InitMongoDB() *mongo.Client {
	logger := logs.GetLogger()
	host := config.GetString("mongodb.host")
	user := config.GetString("mongodb.user")
	password := url.PathEscape(config.GetString("mongodb.password"))

	// mongodb+srv:// resolves the replica set via a DNS SRV record — that's
	// how Atlas advertises its hosts, but a raw IP (self-hosted, no DNS
	// record) has no SRV entry to resolve and must use the plain scheme
	// with an explicit port instead. authSource=admin is required in both
	// cases: every mongo user, Atlas or self-hosted, is provisioned on the
	// admin database regardless of which database it has roles on.
	var dbURL string
	if net.ParseIP(host) != nil {
		dbURL = fmt.Sprintf("mongodb://%s:%s@%s:27017/?retryWrites=true&w=majority&authSource=admin&directConnection=true",
			user, password, host)
	} else {
		dbURL = fmt.Sprintf("mongodb+srv://%s:%s@%s/?retryWrites=true&w=majority&authSource=admin",
			user, password, host)
	}
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(dbURL).SetServerAPIOptions(serverAPI).SetMonitor(otelmongo.NewMonitor(otelmongo.WithCommandAttributeDisabled(false)))

	// Pool settings are optional — when a key is absent from config, leave
	// the corresponding option unset so the driver's own default applies.
	if config.IsSet("mongodb.maxPoolSize") {
		opts.SetMaxPoolSize(uint64(config.GetInt("mongodb.maxPoolSize")))
	}
	if config.IsSet("mongodb.minPoolSize") {
		opts.SetMinPoolSize(uint64(config.GetInt("mongodb.minPoolSize")))
	}
	if config.IsSet("mongodb.maxIdleTimeMS") {
		opts.SetMaxConnIdleTime(time.Duration(config.GetInt("mongodb.maxIdleTimeMS")) * time.Millisecond)
	}

	// appName surfaces in Atlas connection/profiler logs so per-service
	// traffic is identifiable. Only set when the top-level serviceName key
	// is present — no fallback, so its absence leaves the driver default.
	if config.IsSet("serviceName") {
		opts.SetAppName(config.GetString("serviceName"))
	}

	client, err := mongo.Connect(opts)
	if err != nil {
		logger.Fatal("Error connecting to mongo client", zap.Error(err))
		return nil
	}

	logger.Info("You are successfully connected to MongoDB!")

	return client
}
