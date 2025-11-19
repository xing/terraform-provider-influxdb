package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &BucketResource{}
var _ resource.ResourceWithImportState = &BucketResource{}

func NewBucketResource() resource.Resource {
	return &BucketResource{}
}

// ProviderData contains the client and configuration for resources and data sources
type ProviderData struct {
	Client influxdb2.Client
	Org    string
	Bucket string
}

// BucketResource defines the resource implementation.
type BucketResource struct {
	client influxdb2.Client
	org    string
}

// BucketResourceModel describes the resource data model.
type BucketResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Org         types.String `tfsdk:"org"`
	Description types.String `tfsdk:"description"`
}

func (r *BucketResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (r *BucketResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "InfluxDB bucket resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Bucket ID",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Bucket name",
			},
			"org": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization name or ID. If not provided, uses the provider default.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Bucket description",
			},
		},
	}
}

func (r *BucketResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = providerData.Client
	r.org = providerData.Org
}

func (r *BucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BucketResourceModel

	// Read Terraform plan data into the model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use provider org if not specified
	orgName := r.org
	if !data.Org.IsNull() {
		orgName = data.Org.ValueString()
	}

	// Resolve organization name to ID
	orgsAPI := r.client.OrganizationsAPI()
	org, err := orgsAPI.FindOrganizationByName(ctx, orgName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find organization '%s', got error: %s", orgName, err))
		return
	}

	// Create bucket
	bucket := &domain.Bucket{
		Name:  data.Name.ValueString(),
		OrgID: org.Id,
	}

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		bucket.Description = &desc
	}

	bucketsAPI := r.client.BucketsAPI()
	createdBucket, err := bucketsAPI.CreateBucket(ctx, bucket)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create bucket, got error: %s", err))
		return
	}

	// Save data into Terraform state
	data.ID = types.StringValue(*createdBucket.Id)
	data.Name = types.StringValue(createdBucket.Name)
	data.Org = types.StringValue(orgName) // Keep the original organization name/identifier that was used in config
	if createdBucket.Description != nil {
		data.Description = types.StringValue(*createdBucket.Description)
	}

	setDiags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(setDiags...)
}

func (r *BucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BucketResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get bucket by ID
	bucketsAPI := r.client.BucketsAPI()
	bucket, err := bucketsAPI.FindBucketByID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read bucket, got error: %s", err))
		return
	}

	// Update data from API response
	data.Name = types.StringValue(bucket.Name)

	orgsAPI := r.client.OrganizationsAPI()
	org, err := orgsAPI.FindOrganizationByID(ctx, *bucket.OrgID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find organization with ID '%s', got error: %s", *bucket.OrgID, err))
		return
	}
	data.Org = types.StringValue(org.Name)

	if bucket.Description != nil {
		data.Description = types.StringValue(*bucket.Description)
	} else {
		data.Description = types.StringNull()
	}

	readSetDiags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(readSetDiags...)
}

func (r *BucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BucketResourceModel

	// Read Terraform plan data into the model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update bucket
	bucket := &domain.Bucket{
		Id:   data.ID.ValueStringPointer(),
		Name: data.Name.ValueString(),
	}

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		bucket.Description = &desc
	}

	bucketsAPI := r.client.BucketsAPI()
	updatedBucket, err := bucketsAPI.UpdateBucket(ctx, bucket)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update bucket, got error: %s", err))
		return
	}

	// Update data from API response
	data.Name = types.StringValue(updatedBucket.Name)
	if updatedBucket.Description != nil {
		data.Description = types.StringValue(*updatedBucket.Description)
	}

	updateSetDiags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(updateSetDiags...)
}

func (r *BucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BucketResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete bucket
	bucketsAPI := r.client.BucketsAPI()
	err := bucketsAPI.DeleteBucket(ctx, &domain.Bucket{Id: data.ID.ValueStringPointer()})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete bucket, got error: %s", err))
		return
	}
}

func (r *BucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import using bucket ID
	diags := resp.State.SetAttribute(ctx, path.Root("id"), req.ID)
	resp.Diagnostics.Append(diags...)
}
