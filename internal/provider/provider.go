package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/new-work/influxdb-provider/internal/resources"
)

// Ensure InfluxDBProvider satisfies various provider interfaces.
var _ provider.Provider = &InfluxDBProvider{}

// InfluxDBProvider defines the provider implementation.
type InfluxDBProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// tests.
	version string
}

// InfluxDBProviderModel describes the provider data model.
type InfluxDBProviderModel struct {
	URL    types.String `tfsdk:"url"`
	Token  types.String `tfsdk:"token"`
	Org    types.String `tfsdk:"org"`
	Bucket types.String `tfsdk:"bucket"`
}

func (p *InfluxDBProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "influxdb"
	resp.Version = p.version
}

func (p *InfluxDBProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				MarkdownDescription: "InfluxDB URL",
				Optional:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "InfluxDB Token",
				Optional:            true,
				Sensitive:           true,
			},
			"org": schema.StringAttribute{
				MarkdownDescription: "InfluxDB Organization",
				Optional:            true,
			},
			"bucket": schema.StringAttribute{
				MarkdownDescription: "Default InfluxDB Bucket",
				Optional:            true,
			},
		},
	}
}

func (p *InfluxDBProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data InfluxDBProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.
	// Example client configuration for data sources and resources
	url := os.Getenv("INFLUXDB_URL")
	token := os.Getenv("INFLUXDB_TOKEN")
	org := os.Getenv("INFLUXDB_ORG")
	bucket := os.Getenv("INFLUXDB_BUCKET")

	if !data.URL.IsNull() {
		url = data.URL.ValueString()
	}

	if !data.Token.IsNull() {
		token = data.Token.ValueString()
	}

	if !data.Org.IsNull() {
		org = data.Org.ValueString()
	}

	if !data.Bucket.IsNull() {
		bucket = data.Bucket.ValueString()
	}

	if url == "" {
		resp.Diagnostics.AddError(
			"Missing InfluxDB URL",
			"The provider cannot create the InfluxDB client as there is a missing or empty value for the InfluxDB URL. "+
				"Set the url value in the configuration or use the INFLUXDB_URL environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if token == "" {
		resp.Diagnostics.AddError(
			"Missing InfluxDB Token",
			"The provider cannot create the InfluxDB client as there is a missing or empty value for the InfluxDB Token. "+
				"Set the token value in the configuration or use the INFLUXDB_TOKEN environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client := influxdb2.NewClient(url, token)

	// Store client in provider data for use in data sources and resources
	resp.DataSourceData = &resources.ProviderData{
		Client: client,
		Org:    org,
		Bucket: bucket,
	}
	resp.ResourceData = &resources.ProviderData{
		Client: client,
		Org:    org,
		Bucket: bucket,
	}
}

func (p *InfluxDBProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewBucketResource,
		resources.NewTaskResource,
	}
}

func (p *InfluxDBProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// We'll add data sources here later
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &InfluxDBProvider{
			version: version,
		}
	}
}
