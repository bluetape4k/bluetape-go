package jwt

import (
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	defaultMongoCollection  = "jwt_keychains"
	defaultMongoMaxKeyBytes = defaultRedisMaxKeyBytes
	minMongoMaxKeyBytes     = minRedisMaxKeyBytes
	maxMongoMaxKeyBytes     = maxRedisMaxKeyBytes
)

// MongoRepositoryOptions configures a MongoDB-backed distributed KeyChain repository.
type MongoRepositoryOptions struct {
	// Client is caller-owned. The repository never closes it.
	Client *mongo.Client
	// Database is the MongoDB database that stores JWT signing authority.
	Database string
	// Collection is the MongoDB collection that stores current and retained keys.
	Collection string
	// Namespace scopes MongoDB signing authority keys.
	Namespace string
	// Capacity limits retained KeyChains.
	Capacity int
	// MaxKeyBytes limits each serialized KeyChain payload.
	MaxKeyBytes int
}

type mongoRepositoryOptions struct {
	client      *mongo.Client
	database    string
	collection  string
	namespace   string
	capacity    int
	maxKeyBytes int
}

func (o MongoRepositoryOptions) normalize() (mongoRepositoryOptions, error) {
	if o.Client == nil {
		return mongoRepositoryOptions{}, OptionError{Option: "client", Err: errorsNew("must not be nil")}
	}
	database := strings.TrimSpace(o.Database)
	if database == "" {
		return mongoRepositoryOptions{}, OptionError{Option: "database", Err: errorsNew("must not be empty")}
	}
	if err := validateMongoName("database", database); err != nil {
		return mongoRepositoryOptions{}, err
	}
	collection := strings.TrimSpace(o.Collection)
	if collection == "" {
		collection = defaultMongoCollection
	}
	if err := validateMongoName("collection", collection); err != nil {
		return mongoRepositoryOptions{}, err
	}
	namespace, err := normalizeRedisNamespace(o.Namespace)
	if err != nil {
		return mongoRepositoryOptions{}, err
	}
	capacity := o.Capacity
	if capacity == 0 {
		capacity = defaultRepositorySize
	}
	if capacity < minRepositorySize || capacity > maxRepositorySize {
		return mongoRepositoryOptions{}, OptionError{Option: "capacity", Err: errorsNew("outside repository capacity bounds")}
	}
	maxKeyBytes := o.MaxKeyBytes
	if maxKeyBytes == 0 {
		maxKeyBytes = defaultMongoMaxKeyBytes
	}
	if maxKeyBytes < minMongoMaxKeyBytes || maxKeyBytes > maxMongoMaxKeyBytes {
		return mongoRepositoryOptions{}, OptionError{Option: "max_key_bytes", Err: errorsNew("outside mongo key payload bounds")}
	}
	return mongoRepositoryOptions{
		client:      o.Client,
		database:    database,
		collection:  collection,
		namespace:   namespace,
		capacity:    capacity,
		maxKeyBytes: maxKeyBytes,
	}, nil
}

func validateMongoName(option string, value string) error {
	if strings.ContainsAny(value, "\x00/\\.$ ") {
		return OptionError{Option: option, Err: fmt.Errorf("must not contain null, slash, backslash, dot, dollar, or space")}
	}
	return nil
}
