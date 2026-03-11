package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	Postgres PostgresConfig `yaml:"postgres"`
	S3       S3Config       `yaml:"s3"`
	RabbitMQ RabbitMQConfig `yaml:""rabbitmq`
}

type AppConfig struct {
	Name string `yaml:"name"`
	Env  string `yaml:"env"`
	Host string `yaml:"host"`
	Port string `yaml:"port"`
}

type PostgresConfig struct {
	DSN string `yaml:"dsn"`
}

type S3Config struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Bucket    string `yaml:"bucket"`
	UseSSL    bool   `yaml:"use_ssl"`
	Region    string `yaml:"region"`
}

type RabbitMQConfig struct {
	URL              string `yaml:"url"`
	Exchange         string `yaml:"exchange"`
	ExchangeType     string `yaml:"exchange_type"`
	UploadRoutingKey string `yaml:"upload_routing_key"`
	DeleteRoutingKey string `yaml:"delete_routing_key"`
	QueueUpload      string `yaml:"queue_upload"`
	QueueDelete      string `yaml:"queue_delete"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
