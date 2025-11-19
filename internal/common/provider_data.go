package common

import (
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

type ProviderData struct {
	Client influxdb2.Client
	Org    string
	Bucket string
	Token  string
	URL    string
}
